package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
)

func TestPasswordRegistrationRequiresVerificationEmailConfiguration(t *testing.T) {
	server := New(nil, Config{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(
		`{"shopName":"Corner Shop","fullName":"Owner","email":"Owner@Example.com","password":"long-enough-password"}`,
	))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "email_verification_unavailable") {
		t.Fatalf("response = %d %s, want verification email configuration error", response.Code, response.Body.String())
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("signup without verification email configuration unexpectedly set a session cookie")
	}
}

func TestSecurityHeadersAreSet(t *testing.T) {
	server := New(nil, Config{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Cache-Control":          "no-store",
	} {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestFrontendFilesHaveBrowserContentTypes(t *testing.T) {
	frontendDir := t.TempDir()
	files := map[string]string{
		"index.html":              "<!doctype html><title>RetailOS</title>",
		"assets/app.js":           "console.log('app')",
		"assets/app.css":          "body { color: red }",
		"manifest.webmanifest":    `{"name":"RetailOS"}`,
	}
	for name, content := range files {
		filePath := filepath.Join(frontendDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatalf("create frontend directory: %v", err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			t.Fatalf("write frontend file: %v", err)
		}
	}

	server := New(nil, Config{FrontendDir: frontendDir}).Handler()
	tests := []struct {
		path        string
		contentType string
	}{
		{path: "/", contentType: "text/html; charset=utf-8"},
		{path: "/assets/app.js", contentType: "text/javascript; charset=utf-8"},
		{path: "/assets/app.css", contentType: "text/css; charset=utf-8"},
		{path: "/manifest.webmanifest", contentType: "application/manifest+json"},
		{path: "/unknown-route", contentType: "text/html; charset=utf-8"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Content-Type"); got != test.contentType {
				t.Errorf("Content-Type = %q, want %q", got, test.contentType)
			}
		})
	}
}

func TestAuthSessionReturnsGuestWhenCookieIsAbsent(t *testing.T) {
	server := New(nil, Config{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"user":null}` {
		t.Fatalf("status = %d, body = %s; want guest session response", response.Code, response.Body.String())
	}
}

func TestUpdateProfileChangesAuthenticatedUsersName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const (
		shopID = "shop-id"
		userID = "user-id"
	)
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	expectActiveSession(mock, shopID, userID)
	mock.ExpectQuery(`UPDATE users SET full_name = \$1\s+WHERE id = \$2 AND shop_id = \$3 AND active = TRUE\s+RETURNING full_name`).
		WithArgs("Alex Owner", userID, shopID).
		WillReturnRows(sqlmock.NewRows([]string{"full_name"}).AddRow("Alex Owner"))

	request := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", strings.NewReader(`{"fullName":" Alex Owner "}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
	response := httptest.NewRecorder()
	New(db, config).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"fullName":"Alex Owner"}` {
		t.Fatalf("profile update response = %d %s, want updated owner name", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("profile update expectations: %v", err)
	}
}

func TestUpdateProfileRejectsBlankName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const (
		shopID = "shop-id"
		userID = "user-id"
	)
	config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
	expectActiveSession(mock, shopID, userID)

	request := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", strings.NewReader(`{"fullName":"   "}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, shopID, userID, "owner"))
	response := httptest.NewRecorder()
	New(db, config).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_profile") {
		t.Fatalf("blank profile update response = %d %s, want invalid profile", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("blank profile update expectations: %v", err)
	}
}

func TestProductWriteRequiresOwnerOrManager(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT active FROM users WHERE id = \$1 AND shop_id = \$2`).
		WithArgs("user-id", "shop-id").
		WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(true))
	server := New(db, Config{JWTSecret: secret})
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, sessionClaims{
		ShopID: "shop-id",
		Role:   "cashier",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cm-retailos-online",
			Subject:   "user-id",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(secret)
	if err != nil {
		t.Fatalf("sign test session: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("session and permission expectations: %v", err)
	}
}

func TestOwnerManagerReadHandlersRejectCashier(t *testing.T) {
	for _, path := range []string{
		"/api/v1/business-intelligence",
		"/api/v1/audit-logs",
		"/api/v1/settings",
		"/api/v1/settings/preferences",
	} {
		t.Run(path, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("create SQL mock: %v", err)
			}
			defer db.Close()

			config := Config{JWTSecret: []byte("01234567890123456789012345678901")}
			server := New(db, config)
			expectActiveSession(mock, "shop-id", "user-id")

			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.AddCookie(testSessionCookie(t, config, "shop-id", "user-id", "cashier"))
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusForbidden, response.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("cashier read guard expectations: %v", err)
			}
		})
	}
}

func TestStockAdjustmentRequiresOwnerOrManager(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT active FROM users WHERE id = \$1 AND shop_id = \$2`).
		WithArgs("cashier-id", "shop-id").
		WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(true))
	config := Config{JWTSecret: secret}
	server := New(db, config)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/stock-adjustments",
		strings.NewReader(`{"delta":1,"reason":"Security verification"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(testSessionCookie(t, config, "shop-id", "cashier-id", "cashier"))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("cashier stock-adjustment expectations: %v", err)
	}
}

func TestBootstrapRouteDoesNotExist(t *testing.T) {
	server := New(nil, Config{JWTSecret: []byte("01234567890123456789012345678901")})
	bootstrapRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/bootstrap", nil)
	bootstrapResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(bootstrapResponse, bootstrapRequest)
	if bootstrapResponse.Code != http.StatusNotFound {
		t.Fatalf("bootstrap status = %d, want %d", bootstrapResponse.Code, http.StatusNotFound)
	}
}

func TestProtectedRequestRejectsInactiveSession(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT active FROM users WHERE id = \$1 AND shop_id = \$2`).
		WithArgs("user-id", "shop-id").
		WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(false))

	server := New(db, Config{JWTSecret: secret})
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, sessionClaims{
		ShopID: "shop-id",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cm-retailos-online",
			Subject:   "user-id",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(secret)
	if err != nil {
		t.Fatalf("sign test session: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("session validation expectations: %v", err)
	}
}

func TestRetailRoutesRequireSession(t *testing.T) {
	server := New(nil, Config{JWTSecret: []byte("01234567890123456789012345678901")})
	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/auth/me"},
		{method: http.MethodGet, path: "/api/v1/dashboard"},
		{method: http.MethodGet, path: "/api/v1/business-intelligence"},
		{method: http.MethodGet, path: "/api/v1/audit-logs"},
		{method: http.MethodGet, path: "/api/v1/settings"},
		{method: http.MethodPut, path: "/api/v1/settings"},
		{method: http.MethodGet, path: "/api/v1/team-members"},
		{method: http.MethodPost, path: "/api/v1/team-members"},
		{method: http.MethodPatch, path: "/api/v1/team-members/00000000-0000-0000-0000-000000000001/status"},
		{method: http.MethodGet, path: "/api/v1/categories"},
		{method: http.MethodPost, path: "/api/v1/categories"},
		{method: http.MethodPut, path: "/api/v1/categories"},
		{method: http.MethodDelete, path: "/api/v1/categories"},
		{method: http.MethodGet, path: "/api/v1/products"},
		{method: http.MethodPost, path: "/api/v1/products"},
		{method: http.MethodPut, path: "/api/v1/products/1"},
		{method: http.MethodPost, path: "/api/v1/products/1/stock-adjustments"},
		{method: http.MethodPost, path: "/api/v1/products/1/archive"},
		{method: http.MethodPost, path: "/api/v1/sales/checkout"},
		{method: http.MethodGet, path: "/api/v1/sales"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusUnauthorized, response.Body.String())
			}
		})
	}
}

func TestAuthRateLimitReturns429(t *testing.T) {
	server := New(nil, Config{JWTSecret: []byte("01234567890123456789012345678901")})
	for i := 0; i < 31; i++ {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google", nil)
		request.RemoteAddr = "198.51.100.42:1234"
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if i == 30 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("request 31 status = %d, want %d; body: %s", response.Code, http.StatusTooManyRequests, response.Body.String())
			}
			if response.Header().Get("Retry-After") == "" {
				t.Fatal("rate-limited response must include Retry-After")
			}
		}
	}
}

func TestGoogleSignInIsDisabledUntilConfigured(t *testing.T) {
	server := New(nil, Config{JWTSecret: []byte("01234567890123456789012345678901")})

	providersRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers", nil)
	providersResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(providersResponse, providersRequest)
	var providers map[string]bool
	if err := json.Unmarshal(providersResponse.Body.Bytes(), &providers); err != nil {
		t.Fatalf("decode provider response: %v", err)
	}
	if providersResponse.Code != http.StatusOK || providers["googleEnabled"] || !providers["emailEnabled"] ||
		providers["emailVerificationEnabled"] || providers["passwordResetEnabled"] {
		t.Fatalf("provider response = %d %s, want email enabled and mail-dependent features disabled", providersResponse.Code, providersResponse.Body.String())
	}

	googleRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google", nil)
	googleResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(googleResponse, googleRequest)
	if googleResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", googleResponse.Code, http.StatusServiceUnavailable)
	}
}

func TestQuantityAndAmountPrecision(t *testing.T) {
	for _, value := range []float64{0, 0.001, 12.345, 99999999999.999} {
		if !isValidQuantity(value) {
			t.Errorf("isValidQuantity(%v) = false, want true", value)
		}
	}
	for _, value := range []float64{-0.001, 0.0001, 100000000000} {
		if isValidQuantity(value) {
			t.Errorf("isValidQuantity(%v) = true, want false", value)
		}
	}
	for _, value := range []float64{-1, -0.001, 0.001, 1} {
		if !isValidQuantityDelta(value) {
			t.Errorf("isValidQuantityDelta(%v) = false, want true", value)
		}
	}
	for _, value := range []float64{0.0001, 100000000000} {
		if isValidQuantityDelta(value) {
			t.Errorf("isValidQuantityDelta(%v) = true, want false", value)
		}
	}
	for _, value := range []float64{0, 0.01, 12.34} {
		if !isValidAmount(value) {
			t.Errorf("isValidAmount(%v) = false, want true", value)
		}
	}
	for _, value := range []float64{0.001, 1.234} {
		if isValidAmount(value) {
			t.Errorf("isValidAmount(%v) = true, want false", value)
		}
	}
}
