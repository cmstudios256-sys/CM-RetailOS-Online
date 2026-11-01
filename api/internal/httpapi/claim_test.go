package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGuestClaimRequiresSession(t *testing.T) {
	server := New(nil, Config{JWTSecret: []byte("01234567890123456789012345678901")})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/account/claim", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
}

func TestGuestClaimRejectsUnsupportedSchemaVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	expectActiveSession(mock, "shop-id", "user-id")
	server := New(db, config)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/account/claim", strings.NewReader(`{"schemaVersion":2}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, "shop-id", "user-id", "owner"))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_guest_claim") {
		t.Fatalf("status = %d, want invalid bundle response; body: %s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("session validation expectations: %v", err)
	}
}

func TestValidateGuestClaimAcceptsNegativeStockMovement(t *testing.T) {
	input := guestClaimRequest{
		SchemaVersion:  1,
		GuestDeviceID:  "b4cfe4c2-139b-4a2d-b0a2-742c717eb109",
		IdempotencyKey: "a7fd7b2c-2143-46ad-b3a1-ae35989a2d45",
		Products: []guestClaimProduct{{
			ID: 1, Name: "Sample Item", SellingPrice: 10, Quantity: 1,
		}},
		StockMovements: []guestClaimMovement{{
			ID: "guest-stock-sale", ProductID: 1, QuantityDelta: -1,
			Reason: "Sale", CreatedAt: time.Now().UTC(),
		}},
		Settings: guestClaimSettings{
			ShopName: "My Shop", Currency: "UGX", Preferences: map[string]any{},
		},
	}

	if err := validateGuestClaim(input); err != nil {
		t.Fatalf("validateGuestClaim rejected a valid negative stock movement: %v", err)
	}
}

func TestGuestClaimCommitsAndReturnsIdempotentResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-id"
	const userID = "user-id"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	input := guestClaimRequest{
		SchemaVersion:  1,
		GuestDeviceID:  "b4cfe4c2-139b-4a2d-b0a2-742c717eb109",
		IdempotencyKey: "a7fd7b2c-2143-46ad-b3a1-ae35989a2d45",
		Products:       []guestClaimProduct{},
		Categories:     []string{},
		Sales:          []guestClaimSale{},
		StockMovements: []guestClaimMovement{},
		Settings: guestClaimSettings{
			ShopName: "My Shop", Currency: "UGX", Preferences: map[string]any{},
		},
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("encode guest bundle: %v", err)
	}
	hash := sha256.Sum256(payload)
	request := func() *http.Request {
		result := httptest.NewRequest(http.MethodPost, "/api/v1/account/claim", strings.NewReader(string(payload)))
		result.Header.Set("Content-Type", "application/json")
		result.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
		return result
	}

	expectActiveSession(mock, shopID, userID)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO guest_imports`).
		WithArgs(shopID, input.GuestDeviceID, input.IdempotencyKey, hash[:]).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT payload_hash, response`).
		WithArgs(shopID, input.GuestDeviceID, input.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{"payload_hash", "response"}).AddRow(hash[:], nil))
	mock.ExpectExec(`UPDATE shops SET name`).
		WithArgs("My Shop", "UGX", "", "", "", "", false, []byte("{}"), shopID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE guest_imports SET response`).
		WithArgs(sqlmock.AnyArg(), shopID, input.GuestDeviceID, input.IdempotencyKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	firstResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(firstResponse, request())
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d; body: %s", firstResponse.Code, http.StatusCreated, firstResponse.Body.String())
	}
	var firstResult guestClaimResponse
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &firstResult); err != nil {
		t.Fatalf("decode first claim result: %v", err)
	}
	if firstResult.Imported != (guestClaimCounts{}) {
		t.Fatalf("first import counts = %+v, want empty counts", firstResult.Imported)
	}

	expectActiveSession(mock, shopID, userID)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO guest_imports`).
		WithArgs(shopID, input.GuestDeviceID, input.IdempotencyKey, hash[:]).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT payload_hash, response`).
		WithArgs(shopID, input.GuestDeviceID, input.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{"payload_hash", "response"}).
			AddRow(hash[:], []byte(`{"imported":{"products":0,"categories":0,"sales":0,"stockMovements":0},"productIds":{},"saleIds":{}}`)))
	mock.ExpectCommit()

	retryResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(retryResponse, request())
	if retryResponse.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want %d; body: %s", retryResponse.Code, http.StatusOK, retryResponse.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("guest claim expectations: %v", err)
	}
}
