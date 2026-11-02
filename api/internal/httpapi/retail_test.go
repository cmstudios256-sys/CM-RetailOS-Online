package httpapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
)

func testSessionCookie(t *testing.T, config Config, shopID, userID, role string) *http.Cookie {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, sessionClaims{
		ShopID: shopID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cm-retailos-online",
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(config.JWTSecret)
	if err != nil {
		t.Fatalf("sign test session: %v", err)
	}
	return &http.Cookie{Name: sessionCookieName, Value: token}
}

func expectActiveSession(mock sqlmock.Sqlmock, shopID, userID string) {
	mock.ExpectQuery(`SELECT active FROM users WHERE id = \$1 AND shop_id = \$2`).
		WithArgs(userID, shopID).
		WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(true))
}

func TestDashboardRejectsUnsupportedPeriod(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, "shop-a", "user-a")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?period=yearly", nil)
	request.AddCookie(testSessionCookie(t, config, "shop-a", "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected dashboard database request: %v", err)
	}
}

func TestDashboardRejectsInvalidReportDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, "shop-a", "user-a")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?period=Daily&reportDate=2026-02-30", nil)
	request.AddCookie(testSessionCookie(t, config, "shop-a", "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected dashboard database request: %v", err)
	}
}

func TestBusinessIntelligenceReturnsShopScopedPeriodInsights(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, "user-a")
	mock.ExpectQuery(`(?s)SELECT EXTRACT\(HOUR FROM completed_at AT TIME ZONE 'Africa/Kampala'\)::int, count\(\*\).*FROM sales.*shop_id = \$1`).
		WithArgs(shopID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"hour", "count"}).
			AddRow(6, 3).AddRow(23, 2).AddRow(5, 1))
	mock.ExpectQuery(`(?s)SELECT si\.product_name, sum\(si\.line_total - si\.quantity \* si\.unit_cost -.*WHERE s\.shop_id = \$1`).
		WithArgs(shopID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"product_name", "profit"}).
			AddRow("Coffee", 18000.0))
	mock.ExpectQuery(`(?s)WITH sale_products AS \(.*SELECT p1\.product_name, p2\.product_name, count\(\*\).*LIMIT 9`).
		WithArgs(shopID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"first", "second", "frequency"}).
			AddRow("Coffee", "Sugar", 2))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/business-intelligence?period=Weekly&date=2026-10-30", nil)
	request.AddCookie(testSessionCookie(t, config, shopID, "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		PeakSalesHours         []int `json:"peakSalesHours"`
		MostProfitableProducts []struct {
			Name   string  `json:"name"`
			Profit float64 `json:"profit"`
		} `json:"mostProfitableProducts"`
		MarketBasket []struct {
			Pair      []string `json:"pair"`
			Frequency int      `json:"frequency"`
		} `json:"marketBasket"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode business intelligence response: %v", err)
	}
	if len(payload.PeakSalesHours) != 12 || payload.PeakSalesHours[0] != 3 ||
		payload.PeakSalesHours[8] != 2 || payload.PeakSalesHours[11] != 1 {
		t.Fatalf("unexpected peak sales hour buckets: %+v", payload.PeakSalesHours)
	}
	if len(payload.MostProfitableProducts) != 1 || payload.MostProfitableProducts[0].Name != "Coffee" ||
		payload.MostProfitableProducts[0].Profit != 18000 {
		t.Fatalf("unexpected profitable product list: %+v", payload.MostProfitableProducts)
	}
	if len(payload.MarketBasket) != 1 || len(payload.MarketBasket[0].Pair) != 2 ||
		payload.MarketBasket[0].Pair[0] != "Coffee" || payload.MarketBasket[0].Pair[1] != "Sugar" ||
		payload.MarketBasket[0].Frequency != 2 {
		t.Fatalf("unexpected market basket pairs: %+v", payload.MarketBasket)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("business intelligence query expectations: %v", err)
	}
}

func TestAuditLogsReturnsShopScopedRecordedActivity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, "user-a")
	mock.ExpectQuery(`(?s)WITH activity AS \(.*FROM sales s\s+WHERE s\.shop_id = \$1.*FROM stock_movements sm.*WHERE sm\.shop_id = \$1.*LIMIT 500`).
		WithArgs(shopID, "stock").
		WillReturnRows(sqlmock.NewRows([]string{"id", "happened_at", "username", "action", "details", "level"}).
			AddRow("stock-1", time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC), "Shop owner", "Stock adjusted", "Rice: +4 kg (Stock count)", "AUDIT"))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?q=stock", nil)
	request.AddCookie(testSessionCookie(t, config, shopID, "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		Logs []struct {
			ID        string `json:"id"`
			User      string `json:"user"`
			Action    string `json:"action"`
			Details   string `json:"details"`
			Level     string `json:"level"`
			Timestamp string `json:"timestamp"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode audit activity response: %v", err)
	}
	if len(payload.Logs) != 1 || payload.Logs[0].ID != "stock-1" ||
		payload.Logs[0].User != "Shop owner" || payload.Logs[0].Action != "Stock adjusted" ||
		payload.Logs[0].Level != "AUDIT" || payload.Logs[0].Timestamp == "" {
		t.Fatalf("unexpected audit activity response: %+v", payload.Logs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("audit activity query expectations: %v", err)
	}
}

func TestGetShopSettingsScopesQueryToAuthenticatedShop(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, "user-a")
	mock.ExpectQuery(`SELECT name, currency, address, phone, receipt_header, receipt_footer, receipt_show_phone FROM shops WHERE id = \$1`).
		WithArgs(shopID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "currency", "address", "phone", "receipt_header", "receipt_footer", "receipt_show_phone"}).
			AddRow("Corner Shop", "UGX", "Kampala", "+256700000000", "Welcome", "Thank you", true))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	request.AddCookie(testSessionCookie(t, config, shopID, "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		Settings shopSettings `json:"settings"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode settings response: %v", err)
	}
	if payload.Settings.ShopName != "Corner Shop" || payload.Settings.Currency != "UGX" ||
		payload.Settings.Address != "Kampala" || payload.Settings.Phone != "+256700000000" ||
		payload.Settings.ReceiptHeader != "Welcome" || payload.Settings.ReceiptFooter != "Thank you" ||
		!payload.Settings.ReceiptShowPhone {
		t.Fatalf("unexpected shop settings: %+v", payload.Settings)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("settings query expectations: %v", err)
	}
}

func TestUpdateShopSettingsValidatesAndPersistsAllowedFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, "user-a")
	mock.ExpectQuery(`(?s)UPDATE shops SET name = \$1, currency = \$2, address = \$3, phone = \$4,.*WHERE id = \$8\s+RETURNING name, currency, address, phone, receipt_header, receipt_footer, receipt_show_phone`).
		WithArgs("My Retail Shop", "USD", "Main Street", "+1234567890", "Welcome!", "Come again", false, shopID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "currency", "address", "phone", "receipt_header", "receipt_footer", "receipt_show_phone"}).
			AddRow("My Retail Shop", "USD", "Main Street", "+1234567890", "Welcome!", "Come again", false))

	body := `{"shopName":" My Retail Shop ","currency":" usd ","address":" Main Street ","phone":"+1234567890","receiptHeader":"Welcome!","receiptFooter":"Come again","receiptShowPhone":false}`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, "user-a", "manager"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		Settings shopSettings `json:"settings"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode settings response: %v", err)
	}
	if payload.Settings.ShopName != "My Retail Shop" || payload.Settings.Currency != "USD" ||
		payload.Settings.Address != "Main Street" || payload.Settings.ReceiptShowPhone {
		t.Fatalf("unexpected updated settings: %+v", payload.Settings)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("settings update expectations: %v", err)
	}
}

func TestUpdateShopSettingsRejectsInvalidCurrencyWithoutDatabaseWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, "shop-a", "user-a")
	body := `{"shopName":"Shop","currency":"US1","address":"","phone":"","receiptHeader":"","receiptFooter":"","receiptShowPhone":true}`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, "shop-a", "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("invalid settings should not be written: %v", err)
	}
}

func TestCashierCannotUpdateShopSettings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, "shop-a", "user-a")
	body := `{"shopName":"Shop","currency":"UGX","address":"","phone":"","receiptHeader":"","receiptFooter":"","receiptShowPhone":true}`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, "shop-a", "user-a", "cashier"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("cashier settings update should not write to database: %v", err)
	}
}

func TestListTeamMembersScopesResultsToOwnerShop(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const userID = "user-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, userID)
	mock.ExpectQuery(`(?s)SELECT id::text, full_name, email, role, active, created_at\s+FROM users\s+WHERE shop_id = \$1`).
		WithArgs(shopID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "email", "role", "active", "created_at"}).
			AddRow("member-1", "Cashier One", "cashier@example.com", "cashier", true, time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/team-members", nil)
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		Members []teamMember `json:"members"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode team member response: %v", err)
	}
	if len(payload.Members) != 1 || payload.Members[0].Email != "cashier@example.com" ||
		payload.Members[0].Role != "cashier" || !payload.Members[0].Active {
		t.Fatalf("unexpected team member list: %+v", payload.Members)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("team member query expectations: %v", err)
	}
}

func TestCreateTeamMemberValidatesAndHashesStaffCredentials(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const userID = "user-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, userID)
	mock.ExpectQuery(`(?s)INSERT INTO users \(shop_id, full_name, email, password_hash, role\).*RETURNING id::text, full_name, email, role, active, created_at`).
		WithArgs(shopID, "Cashier One", "cashier@example.com", sqlmock.AnyArg(), "cashier").
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "email", "role", "active", "created_at"}).
			AddRow("00000000-0000-0000-0000-000000000002", "Cashier One", "cashier@example.com", "cashier", true, time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)))

	body := `{"fullName":" Cashier One ","email":" CASHIER@example.com ","role":"cashier","password":"securepass8"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/team-members", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	var payload struct {
		Member teamMember `json:"member"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode created team member response: %v", err)
	}
	if payload.Member.FullName != "Cashier One" || payload.Member.Email != "cashier@example.com" ||
		payload.Member.Role != "cashier" || !payload.Member.Active {
		t.Fatalf("unexpected created team member: %+v", payload.Member)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("team member creation expectations: %v", err)
	}
}

func TestCreateTeamMemberRejectsOwnerRoleAndShortPassword(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "owner role", body: `{"fullName":"Another Owner","email":"owner2@example.com","role":"owner","password":"securepass8"}`},
		{name: "short password", body: `{"fullName":"Cashier","email":"cashier@example.com","role":"cashier","password":"short"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("create SQL mock: %v", err)
			}
			defer db.Close()
			config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
			server := New(db, config)
			expectActiveSession(mock, "shop-a", "user-a")
			request := httptest.NewRequest(http.MethodPost, "/api/v1/team-members", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(testSessionCookie(t, config, "shop-a", "user-a", "owner"))
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("invalid staff input should not be written: %v", err)
			}
		})
	}
}

func TestManagerCannotManageTeamAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, "shop-a", "manager-a")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/team-members", nil)
	request.AddCookie(testSessionCookie(t, config, "shop-a", "manager-a", "manager"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("manager should not access team accounts: %v", err)
	}
}

func TestUpdateTeamMemberStatusCannotDeactivateLastOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "00000000-0000-0000-0000-000000000010"
	const ownerID = "00000000-0000-0000-0000-000000000001"
	const memberID = "00000000-0000-0000-0000-000000000002"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, ownerID)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id::text FROM shops WHERE id = \$1 FOR UPDATE`).
		WithArgs(shopID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(shopID))
	mock.ExpectQuery(`(?s)SELECT id::text, full_name, email, role, active, created_at\s+FROM users WHERE id = \$1 AND shop_id = \$2 FOR UPDATE`).
		WithArgs(memberID, shopID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "email", "role", "active", "created_at"}).
			AddRow(memberID, "Another Owner", "owner2@example.com", "owner", true, time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)))
	mock.ExpectQuery(`SELECT count\(\*\) FROM users WHERE shop_id = \$1 AND role = 'owner' AND active = TRUE`).
		WithArgs(shopID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/team-members/"+memberID+"/status", strings.NewReader(`{"active":false}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, ownerID, "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "last_active_owner") {
		t.Fatalf("status/body = %d %s, want conflict for last active owner", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("last-owner safeguard expectations: %v", err)
	}
}

func TestDeactivatedTeamMemberSessionIsRejected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	mock.ExpectQuery(`SELECT active FROM users WHERE id = \$1 AND shop_id = \$2`).
		WithArgs("inactive-user", "shop-a").
		WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(false))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	request.AddCookie(testSessionCookie(t, config, "shop-a", "inactive-user", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("inactive session expectations: %v", err)
	}
}

func TestListSalesReturnsHistoryBeyondLatestHundred(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, "user-a")
	mock.ExpectQuery("WITH shop_sales").
		WithArgs(shopID).
		WillReturnRows(func() *sqlmock.Rows {
			rows := sqlmock.NewRows([]string{
				"id", "cashier_name", "customer_name", "subtotal", "discount", "total", "payment_method",
				"completed_at", "product_id", "product_name", "quantity", "unit_price", "unit_cost", "line_total",
			})
			for i := 1; i <= 101; i++ {
				rows.AddRow(fmt.Sprintf("sale-%03d", i), "Cashier", "", 100.0, 0.0, 100.0, "cash",
					time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC), nil, nil, nil, nil, nil, nil)
			}
			return rows
		}())

	request := httptest.NewRequest(http.MethodGet, "/api/v1/sales", nil)
	request.AddCookie(testSessionCookie(t, config, shopID, "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		Sales []sale `json:"sales"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode sales response: %v", err)
	}
	if len(payload.Sales) != 101 {
		t.Fatalf("sales returned = %d, want all 101 records", len(payload.Sales))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sales query expectations: %v", err)
	}
}

func TestDashboardReturnsShopScopedPeriodInsights(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, "user-a")
	mock.ExpectQuery(`(?s)SELECT\s+\(SELECT count\(\*\) FROM products.*SELECT currency FROM shops WHERE id = \$1\)`).
		WithArgs(shopID).
		WillReturnRows(sqlmock.NewRows([]string{
			"product_count", "low_stock_count", "inventory_value", "today_sales", "today_revenue", "currency",
		}).AddRow(3, 1, 42000.0, 2, 15000.0, "UGX"))
	mock.ExpectQuery(`(?s)SELECT s\.completed_at, s\.total, COALESCE\(sum\(si\.quantity\), 0\).*GROUP BY s\.id`).
		WithArgs(shopID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"completed_at", "total", "units", "cost"}).
			AddRow(time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC), 15000.0, 2.0, 9000.0))
	mock.ExpectQuery(`(?s)SELECT COALESCE\(NULLIF\(p\.category, ''\), 'Uncategorized'\).*s\.discount.*GROUP BY COALESCE\(NULLIF\(p\.category, ''\), 'Uncategorized'\)`).
		WithArgs(shopID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"category", "profit"}).
			AddRow("Grocery", 6000.0))
	mock.ExpectQuery(`(?s)SELECT count\(\*\), COALESCE\(sum\(s\.total\), 0\).*FROM sales s.*s\.shop_id = \$1`).
		WithArgs(shopID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "revenue", "profit"}).AddRow(1, 12000.0, 5000.0))
	mock.ExpectQuery(`(?s)SELECT si\.product_name, sum\(si\.quantity\),.*completed_at >= \$2`).
		WithArgs(shopID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"product_name", "quantity_sold", "revenue"}).
			AddRow("Rice", 2.0, 15000.0))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?period=Monthly&reportDate=2026-10-30", nil)
	request.AddCookie(testSessionCookie(t, config, shopID, "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		InventoryValue   float64                   `json:"inventoryValue"`
		Revenue          float64                   `json:"revenue"`
		Profit           float64                   `json:"profit"`
		PreviousRevenue  float64                   `json:"previousRevenue"`
		PreviousProfit   float64                   `json:"previousProfit"`
		OrderCount       int64                     `json:"orderCount"`
		PreviousOrders   int64                     `json:"previousOrders"`
		UnitsSold        float64                   `json:"unitsSold"`
		Chart            []dashboardChartPoint     `json:"chart"`
		TopProducts      []dashboardTopProduct     `json:"topProducts"`
		ProfitByCategory []dashboardProfitCategory `json:"profitByCategory"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode dashboard response: %v", err)
	}
	if payload.InventoryValue != 42000 || payload.Revenue != 15000 || payload.Profit != 6000 ||
		payload.PreviousRevenue != 12000 || payload.PreviousProfit != 5000 ||
		payload.OrderCount != 1 || payload.PreviousOrders != 1 || payload.UnitsSold != 2 {
		t.Fatalf("unexpected dashboard totals: %+v", payload)
	}
	if len(payload.Chart) != 24 || len(payload.TopProducts) != 1 || payload.TopProducts[0].Name != "Rice" ||
		len(payload.ProfitByCategory) != 1 || payload.ProfitByCategory[0].Name != "Grocery" ||
		payload.ProfitByCategory[0].Value != 6000 {
		t.Fatalf("unexpected dashboard series or top products: %+v", payload)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("dashboard query expectations: %v", err)
	}
}

func TestListProductsScopesQueryToAuthenticatedShop(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, "user-a")
	mock.ExpectQuery(`SELECT id, sku, name, category, barcode, unit, selling_price, cost_price, quantity, min_stock, image FROM products WHERE shop_id = \$1 AND active = TRUE.*`).
		WithArgs(shopID, "").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "sku", "name", "category", "barcode", "unit", "selling_price", "cost_price", "quantity", "min_stock", "image",
		}).AddRow(4, "RICE-4", "Rice", "Grocery", "", "kg", "5000.00", "4000.00", "8.000", "2.000", ""))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	request.AddCookie(testSessionCookie(t, config, shopID, "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		Products []product `json:"products"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode product response: %v", err)
	}
	if len(payload.Products) != 1 || payload.Products[0].Name != "Rice" {
		t.Fatalf("unexpected product response: %+v", payload.Products)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL shop-scope expectations: %v", err)
	}
}

func TestListCategoriesScopesResultsToAuthenticatedShop(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, "user-a")
	mock.ExpectQuery(`(?s)SELECT name FROM categories WHERE shop_id = \$1.*SELECT DISTINCT category FROM products WHERE shop_id = \$1`).
		WithArgs(shopID).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Drinks").AddRow("Groceries"))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	request.AddCookie(testSessionCookie(t, config, shopID, "user-a", "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode categories: %v", err)
	}
	if len(payload.Categories) != 2 || payload.Categories[0] != "Drinks" || payload.Categories[1] != "Groceries" {
		t.Fatalf("unexpected categories: %+v", payload.Categories)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("category shop-scope expectations: %v", err)
	}
}

func TestCreateCategoryRequiresManagerAndScopesInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const userID = "user-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, userID)
	mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM categories WHERE shop_id = \$1.*FROM products WHERE shop_id = \$1`).
		WithArgs(shopID, "Beverages").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`INSERT INTO categories \(shop_id, name\) VALUES \(\$1, \$2\)`).
		WithArgs(shopID, "Beverages").
		WillReturnResult(sqlmock.NewResult(1, 1))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/categories", strings.NewReader(`{"name":" Beverages "}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "manager"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("category creation expectations: %v", err)
	}
}

func TestRenameCategoryUpdatesShopProductsAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const userID = "user-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, userID)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM categories WHERE shop_id = \$1.*FROM products WHERE shop_id = \$1`).
		WithArgs(shopID, "Drinks").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM categories WHERE shop_id = \$1.*FROM products WHERE shop_id = \$1`).
		WithArgs(shopID, "Beverages").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`UPDATE products SET category = \$1, updated_at = now\(\).*WHERE shop_id = \$2 AND active = TRUE`).
		WithArgs("Beverages", shopID, "Drinks").
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(`UPDATE categories SET name = \$1 WHERE shop_id = \$2`).
		WithArgs("Beverages", shopID, "Drinks").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	request := httptest.NewRequest(http.MethodPut, "/api/v1/categories", strings.NewReader(`{"oldName":"Drinks","name":"Beverages"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("category rename expectations: %v", err)
	}
}

func TestCheckoutCannotSellProductFromAnotherShop(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const userID = "user-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, userID)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT full_name FROM users WHERE id = \$1 AND shop_id = \$2 AND active = TRUE`).
		WithArgs(userID, shopID).
		WillReturnRows(sqlmock.NewRows([]string{"full_name"}).AddRow("Shop A Owner"))
	mock.ExpectQuery(`UPDATE products SET quantity = quantity - \$1::numeric, updated_at = now\(\) WHERE id = \$2 AND shop_id = \$3 AND active = TRUE AND quantity >= \$1::numeric RETURNING name, selling_price, cost_price`).
		WithArgs(1.0, int64(41), shopID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM products WHERE id = \$1 AND shop_id = \$2 AND active = TRUE\)`).
		WithArgs(int64(41), shopID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sales/checkout",
		strings.NewReader(`{"customerName":"","paymentMethod":"cash","items":[{"productId":41,"quantity":1}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusConflict, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "product_unavailable") {
		t.Fatalf("response does not identify unavailable product: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL shop-isolation expectations: %v", err)
	}
}

func TestCheckoutRejectsDiscountForCashier(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const userID = "cashier-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, userID)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sales/checkout",
		strings.NewReader(`{"customerName":"","paymentMethod":"cash","discount":5,"items":[{"productId":1,"quantity":1}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "cashier"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "discount_not_allowed") {
		t.Fatalf("response = %d %s, want discount permission denial", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("cashier discount expectations: %v", err)
	}
}

func TestCheckoutRejectsNegativeDiscount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const userID = "owner-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, userID)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sales/checkout",
		strings.NewReader(`{"customerName":"","paymentMethod":"cash","discount":-1,"items":[{"productId":1,"quantity":1}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_discount") {
		t.Fatalf("response = %d %s, want invalid discount rejection", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("negative discount expectations: %v", err)
	}
}

func TestCheckoutAppliesManagerDiscountToRecordedSaleTotal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const userID = "manager-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, userID)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT full_name FROM users WHERE id = \$1 AND shop_id = \$2 AND active = TRUE`).
		WithArgs(userID, shopID).
		WillReturnRows(sqlmock.NewRows([]string{"full_name"}).AddRow("Shop A Manager"))
	mock.ExpectQuery(`UPDATE products SET quantity = quantity - \$1::numeric, updated_at = now\(\).*RETURNING name, selling_price, cost_price`).
		WithArgs(2.0, int64(7), shopID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "selling_price", "cost_price"}).AddRow("Rice", 100.0, 60.0))
	mock.ExpectQuery(`INSERT INTO sales \(shop_id, cashier_id, cashier_name, customer_name, subtotal, discount, total, payment_method\).*RETURNING id::text`).
		WithArgs(shopID, userID, "Shop A Manager", "Customer", 200.0, 20.0, 180.0, "cash").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "cashier_name", "customer_name", "subtotal", "discount", "total", "payment_method", "completed_at",
		}).AddRow("sale-id", "Shop A Manager", "Customer", 200.0, 20.0, 180.0, "cash", time.Now()))
	mock.ExpectExec(`INSERT INTO sale_items`).
		WithArgs("sale-id", int64(7), "Rice", 2.0, 100.0, 60.0, 200.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO stock_movements`).
		WithArgs(shopID, int64(7), userID, -2.0, "Sale sale-id", "sale-id").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sales/checkout",
		strings.NewReader(`{"customerName":"Customer","paymentMethod":"cash","discount":20,"items":[{"productId":7,"quantity":2}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "manager"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	var payload struct {
		Sale sale `json:"sale"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode discounted sale: %v", err)
	}
	if payload.Sale.Subtotal != 200 || payload.Sale.Discount != 20 || payload.Sale.Total != 180 {
		t.Fatalf("unexpected discounted sale totals: %+v", payload.Sale)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("discounted checkout expectations: %v", err)
	}
}

func TestCheckoutRejectsInsufficientStockWithoutCommitting(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const userID = "user-a"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)
	expectActiveSession(mock, shopID, userID)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT full_name FROM users WHERE id = \$1 AND shop_id = \$2 AND active = TRUE`).
		WithArgs(userID, shopID).
		WillReturnRows(sqlmock.NewRows([]string{"full_name"}).AddRow("Shop A Owner"))
	mock.ExpectQuery(`UPDATE products SET quantity = quantity - \$1::numeric, updated_at = now\(\) WHERE id = \$2 AND shop_id = \$3 AND active = TRUE AND quantity >= \$1::numeric RETURNING name, selling_price, cost_price`).
		WithArgs(2.0, int64(7), shopID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM products WHERE id = \$1 AND shop_id = \$2 AND active = TRUE\)`).
		WithArgs(int64(7), shopID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sales/checkout",
		strings.NewReader(`{"customerName":"","paymentMethod":"cash","items":[{"productId":7,"quantity":2}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusConflict, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "insufficient_stock") {
		t.Fatalf("response does not identify insufficient stock: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("checkout rollback expectations: %v", err)
	}
}

func TestSettingsPreferencesAndStaffPermissionsPersistInDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-a"
	const ownerID = "00000000-0000-0000-0000-000000000001"
	const cashierID = "00000000-0000-0000-0000-000000000002"
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	server := New(db, config)

	// 1. PUT /api/v1/settings/preferences
	expectActiveSession(mock, shopID, ownerID)
	mock.ExpectQuery(`(?s)UPDATE shops SET preferences = \(\$1::jsonb - '__businessProfileSetupComplete'\).*OR \$1::jsonb->'__businessProfileSetupComplete' = 'true'::jsonb.*preferences->'__businessProfileSetupComplete'.*WHERE id = \$2\s+RETURNING preferences`).
		WithArgs(sqlmock.AnyArg(), shopID).
		WillReturnRows(sqlmock.NewRows([]string{"preferences"}).
			AddRow([]byte(`{"country":"Uganda","applicationSettings":{"themeMode":"light"},"__businessProfileSetupComplete":true}`)))

	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/preferences",
		strings.NewReader(`{"preferences":{"country":"Uganda","applicationSettings":{"themeMode":"light"},"__businessProfileSetupComplete":true}}`))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.AddCookie(testSessionCookie(t, config, shopID, ownerID, "owner"))
	putResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("PUT preferences status = %d, want %d; body: %s", putResp.Code, http.StatusOK, putResp.Body.String())
	}

	// 2. PUT /api/v1/team-members/{id}/permissions
	expectActiveSession(mock, shopID, ownerID)
	mock.ExpectQuery(`(?s)UPDATE users\s+SET permissions = \$1::jsonb,\s+full_name = CASE WHEN \$2 <> '' THEN \$2 ELSE full_name END\s+WHERE id = \$3 AND shop_id = \$4\s+RETURNING id::text`).
		WithArgs(`["page_pos","page_inventory"]`, "Cashier One", cashierID, shopID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(cashierID))

	permReq := httptest.NewRequest(http.MethodPut, "/api/v1/team-members/"+cashierID+"/permissions",
		strings.NewReader(`{"fullName":"Cashier One","permissions":["page_pos","page_inventory"]}`))
	permReq.Header.Set("Content-Type", "application/json")
	permReq.AddCookie(testSessionCookie(t, config, shopID, ownerID, "owner"))
	permResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(permResp, permReq)
	if permResp.Code != http.StatusOK {
		t.Fatalf("PUT staff permissions status = %d, want %d; body: %s", permResp.Code, http.StatusOK, permResp.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("preferences and permissions expectations: %v", err)
	}
}
