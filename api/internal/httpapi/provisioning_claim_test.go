package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFirstTimeGoogleProvisioningAndGuestClaim(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const (
		shopID = "shop-id"
		userID = "user-id"
		sub    = "google-subject"
	)
	server := New(db, Config{})

	mock.ExpectBegin()
	mock.ExpectQuery(`WHERE u\.google_subject = \$1`).WithArgs(sub).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`WHERE lower\(u\.email\) = lower\(\$1\)`).WithArgs("owner@example.com").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`INSERT INTO shops \(name\) VALUES \(\$1\) RETURNING id::text`).
		WithArgs("My Shop").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(shopID))
	mock.ExpectQuery(`INSERT INTO users \(shop_id, full_name, email, google_subject, google_picture_url, role\)`).
		WithArgs(shopID, "Shop Owner", "owner@example.com", sub, "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "shop_id", "full_name", "email", "role"}).
			AddRow(userID, shopID, "Shop Owner", "owner@example.com", "owner"))
	mock.ExpectCommit()

	user, err := server.findOrCreateGoogleUser(context.Background(), sub, googleIdentityClaims{
		Email: "owner@example.com", EmailVerified: true, Name: "Google Profile",
	}, "")
	if err != nil {
		t.Fatalf("provision first Google account: %v", err)
	}
	if user.UserID != userID || user.ShopID != shopID || user.ShopName != "My Shop" ||
		user.FullName != "Shop Owner" || user.Email != "owner@example.com" ||
		user.Role != "owner" || user.Currency != "UGX" || !user.GoogleConnected {
		t.Fatalf("unexpected first-time Google account and defaults: %+v", user)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`WHERE u\.google_subject = \$1`).WithArgs(sub).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "shop_id", "full_name", "email", "role", "shop_name", "currency", "google_picture_url", "active",
		}).AddRow(userID, shopID, "Shop Owner", "owner@example.com", "owner", "My Shop", "UGX", "", true))
	mock.ExpectCommit()

	matchedUser, err := server.findOrCreateGoogleUser(context.Background(), sub, googleIdentityClaims{
		Email: "a-different-email@example.com", EmailVerified: true,
	}, "An Ignored Shop")
	if err != nil {
		t.Fatalf("find existing Google account by subject: %v", err)
	}
	if matchedUser.UserID != userID || matchedUser.ShopID != shopID || matchedUser.Email != "owner@example.com" {
		t.Fatalf("Google subject did not resolve to the original account: %+v", matchedUser)
	}

	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server = New(db, config)
	input := guestClaimRequest{
		SchemaVersion:  1,
		GuestDeviceID:  "b4cfe4c2-139b-4a2d-b0a2-742c717eb109",
		IdempotencyKey: "a7fd7b2c-2143-46ad-b3a1-ae35989a2d45",
		Products: []guestClaimProduct{{
			ID: 7, Name: "Coffee", SellingPrice: 10, Quantity: 4,
		}},
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
	payloadHash := sha256.Sum256(payload)
	request := func(shopID, userID string, body []byte) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/account/claim", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
		return req
	}
	expectClaimSession := func(claimShopID, claimUserID string) {
		expectActiveSession(mock, claimShopID, claimUserID)
	}
	expectNewClaim := func(claimShopID string, hash []byte, expectedProductID int64) {
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO guest_imports`).
			WithArgs(claimShopID, input.GuestDeviceID, input.IdempotencyKey, hash).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`SELECT payload_hash, response`).
			WithArgs(claimShopID, input.GuestDeviceID, input.IdempotencyKey).
			WillReturnRows(sqlmock.NewRows([]string{"payload_hash", "response"}).AddRow(hash, nil))
		mock.ExpectQuery(`INSERT INTO products`).
			WithArgs(claimShopID, "", "Coffee", "", "", "", 10.0, 0.0, 4.0, 0.0, "", true).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedProductID))
		mock.ExpectExec(`(?s)UPDATE shops SET\s+name = CASE WHEN.*__businessProfileSetupComplete.*receipt_show_phone = CASE WHEN.*preferences = .*WHERE id = \$9`).
			WithArgs("My Shop", "UGX", "", "", "", "", false, []byte("{}"), claimShopID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE guest_imports SET response`).
			WithArgs(sqlmock.AnyArg(), claimShopID, input.GuestDeviceID, input.IdempotencyKey).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
	}

	expectClaimSession(shopID, userID)
	expectNewClaim(shopID, payloadHash[:], 501)
	firstResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(firstResponse, request(shopID, userID, payload))
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first claim status = %d, want %d; body: %s", firstResponse.Code, http.StatusCreated, firstResponse.Body.String())
	}
	var firstResult guestClaimResponse
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &firstResult); err != nil {
		t.Fatalf("decode first claim result: %v", err)
	}
	if firstResult.Imported.Products != 1 || firstResult.ProductIDs["7"] != 501 {
		t.Fatalf("unexpected first claim result: %+v", firstResult)
	}

	expectClaimSession(shopID, userID)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO guest_imports`).
		WithArgs(shopID, input.GuestDeviceID, input.IdempotencyKey, payloadHash[:]).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT payload_hash, response`).
		WithArgs(shopID, input.GuestDeviceID, input.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{"payload_hash", "response"}).
			AddRow(payloadHash[:], firstResponse.Body.Bytes()))
	mock.ExpectCommit()
	retryResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(retryResponse, request(shopID, userID, payload))
	if retryResponse.Code != http.StatusOK || retryResponse.Body.String() != firstResponse.Body.String() {
		t.Fatalf("idempotent claim status/body = %d %s, want original result; body: %s",
			retryResponse.Code, retryResponse.Body.String(), firstResponse.Body.String())
	}

	conflictingInput := input
	conflictingInput.Settings.ShopName = "Changed Shop"
	conflictingPayload, err := json.Marshal(conflictingInput)
	if err != nil {
		t.Fatalf("encode conflicting guest bundle: %v", err)
	}
	conflictingHash := sha256.Sum256(conflictingPayload)
	expectClaimSession(shopID, userID)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO guest_imports`).
		WithArgs(shopID, input.GuestDeviceID, input.IdempotencyKey, conflictingHash[:]).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT payload_hash, response`).
		WithArgs(shopID, input.GuestDeviceID, input.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{"payload_hash", "response"}).
			AddRow(payloadHash[:], firstResponse.Body.Bytes()))
	mock.ExpectRollback()
	conflictResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(conflictResponse, request(shopID, userID, conflictingPayload))
	if conflictResponse.Code != http.StatusConflict ||
		!strings.Contains(conflictResponse.Body.String(), "guest_claim_key_reused") {
		t.Fatalf("conflicting claim status = %d, want payload conflict; body: %s",
			conflictResponse.Code, conflictResponse.Body.String())
	}

	const otherShopID = "other-shop-id"
	const otherUserID = "other-user-id"
	expectClaimSession(otherShopID, otherUserID)
	expectNewClaim(otherShopID, payloadHash[:], 601)
	otherTenantResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(otherTenantResponse, request(otherShopID, otherUserID, payload))
	if otherTenantResponse.Code != http.StatusCreated {
		t.Fatalf("same claim key in another tenant status = %d, want %d; body: %s",
			otherTenantResponse.Code, http.StatusCreated, otherTenantResponse.Body.String())
	}
	var otherTenantResult guestClaimResponse
	if err := json.Unmarshal(otherTenantResponse.Body.Bytes(), &otherTenantResult); err != nil {
		t.Fatalf("decode other-tenant claim result: %v", err)
	}
	if otherTenantResult.ProductIDs["7"] != 601 {
		t.Fatalf("claim was not isolated to other tenant: %+v", otherTenantResult)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("provisioning and guest claim expectations: %v", err)
	}
}
