package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
)

func TestGoogleLoginUsesSignedStateNonceAndPKCE(t *testing.T) {
	config := googleTestConfig()
	server := New(nil, config)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google?shopName=Kisakye%20Store&next=products", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusFound, response.Body.String())
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Google authorization URL: %v", err)
	}
	if location.Host != "accounts.google.com" || location.Path != "/o/oauth2/v2/auth" {
		t.Fatalf("authorization location = %s, want Google OAuth endpoint", location)
	}
	query := location.Query()
	if query.Get("client_id") != config.GoogleClientID ||
		query.Get("redirect_uri") != config.GoogleRedirectURL ||
		query.Get("code_challenge_method") != "S256" ||
		query.Get("scope") != "openid email profile" ||
		query.Get("prompt") != "select_account" {
		t.Fatalf("unexpected OAuth parameters: %v", query)
	}
	if query.Get("code_challenge") == "" || query.Get("nonce") == "" || query.Get("state") == "" {
		t.Fatalf("PKCE, nonce, or state missing from OAuth request: %v", query)
	}

	cookies := response.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("OAuth cookies = %d, want 3", len(cookies))
	}
	values := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		values[cookie.Name] = cookie.Value
		if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
			t.Errorf("OAuth cookie %q has insecure attributes: %+v", cookie.Name, cookie)
		}
	}
	claims := &googleStateClaims{}
	token, err := jwt.ParseWithClaims(query.Get("state"), claims, func(token *jwt.Token) (any, error) {
		return config.JWTSecret, nil
	}, jwt.WithIssuer(googleStateIssuer), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		t.Fatalf("OAuth state is not signed and valid: %v", err)
	}
	if claims.ShopName != "Kisakye Store" || claims.NextPage != "products" || claims.Nonce != values[googleNonceCookie] ||
		values[googleStateCookie] != query.Get("state") || values[googlePKCECookie] == "" {
		t.Fatalf("OAuth state does not match signed shop name and browser cookies: %+v", claims)
	}
}

func TestGoogleLoginRejectsInvalidShopName(t *testing.T) {
	server := New(nil, googleTestConfig())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google?shopName="+strings.Repeat("a", 121), nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("invalid shop name created OAuth state cookies")
	}
}

func TestGoogleCallbackDestinationReturnsNewShopToProducts(t *testing.T) {
	server := New(nil, googleTestConfig())
	destination, err := server.googleCallbackDestination("products")
	if err != nil {
		t.Fatalf("build Google callback destination: %v", err)
	}
	if destination.Query().Get("page") != "products" {
		t.Fatalf("callback destination = %s, want Products page", destination)
	}

	defaultDestination, err := server.googleCallbackDestination("")
	if err != nil {
		t.Fatalf("build default Google callback destination: %v", err)
	}
	if defaultDestination.RawQuery != "" {
		t.Fatalf("default callback destination unexpectedly has a query: %s", defaultDestination)
	}
}

func TestGoogleCallbackRejectsMissingOAuthState(t *testing.T) {
	server := New(nil, googleTestConfig())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=unused&state=missing", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusSeeOther)
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse callback redirect: %v", err)
	}
	if location.Query().Get("google_error") != "google_sign_in_expired" {
		t.Fatalf("callback error = %q, want expired state", location.Query().Get("google_error"))
	}
}

func TestFindOrCreateGoogleUserDefaultsShopNameWhenEmptyForNewAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-id"
	mock.ExpectBegin()
	mock.ExpectQuery(`WHERE u\.google_subject = \$1`).WithArgs("google-subject").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`WHERE lower\(u\.email\) = lower\(\$1\)`).WithArgs("owner@example.com").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`INSERT INTO shops \(name\) VALUES \(\$1\) RETURNING id::text`).
		WithArgs("Owner's Shop").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(shopID))
	mock.ExpectQuery(`INSERT INTO users \(shop_id, full_name, email, google_subject, google_picture_url, role\)`).
		WithArgs(shopID, "Owner", "owner@example.com", "google-subject", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "shop_id", "full_name", "email", "role"}).
			AddRow("user-id", shopID, "Owner", "owner@example.com", "owner"))
	mock.ExpectCommit()

	server := New(db, Config{})
	user, err := server.findOrCreateGoogleUser(context.Background(), "google-subject", googleIdentityClaims{
		Email: "owner@example.com", EmailVerified: true, Name: "Owner",
	}, "")
	if err != nil {
		t.Fatalf("create Google owner without shop name: %v", err)
	}
	if user.ShopName != "Owner's Shop" {
		t.Fatalf("shopName = %q, want %q", user.ShopName, "Owner's Shop")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestFindOrCreateGoogleUserCreatesPasswordlessOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	const shopID = "shop-id"
	const pictureURL = "https://lh3.googleusercontent.com/a/example-photo"
	mock.ExpectBegin()
	mock.ExpectQuery(`WHERE u\.google_subject = \$1`).WithArgs("google-subject").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`WHERE lower\(u\.email\) = lower\(\$1\)`).WithArgs("owner@example.com").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`INSERT INTO shops \(name\) VALUES \(\$1\) RETURNING id::text`).
		WithArgs("New Shop").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(shopID))
	mock.ExpectQuery(`INSERT INTO users \(shop_id, full_name, email, google_subject, google_picture_url, role\)`).
		WithArgs(shopID, "Owner", "owner@example.com", "google-subject", pictureURL).
		WillReturnRows(sqlmock.NewRows([]string{"id", "shop_id", "full_name", "email", "role"}).
			AddRow("user-id", shopID, "Owner", "owner@example.com", "owner"))
	mock.ExpectCommit()

	server := New(db, Config{})
	user, err := server.findOrCreateGoogleUser(context.Background(), "google-subject", googleIdentityClaims{
		Email: "owner@example.com", EmailVerified: true, Name: "Owner", Picture: pictureURL,
	}, "New Shop")
	if err != nil {
		t.Fatalf("create Google owner: %v", err)
	}
	if user.UserID != "user-id" || user.ShopID != shopID || user.ShopName != "New Shop" ||
		user.FullName != "Owner" || user.Email != "owner@example.com" || user.Role != "owner" ||
		user.Currency != "UGX" || user.GooglePictureURL != pictureURL {
		t.Fatalf("unexpected Google owner: %+v", user)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestValidGooglePictureURL(t *testing.T) {
	for _, value := range []string{
		"https://lh3.googleusercontent.com/a/example-photo",
		"https://googleusercontent.com/profile?sz=96",
	} {
		if got := validGooglePictureURL(value); got != value {
			t.Errorf("validGooglePictureURL(%q) = %q", value, got)
		}
	}
	for _, value := range []string{
		"http://lh3.googleusercontent.com/a/photo",
		"https://example.com/photo",
		"https://lh3.googleusercontent.com.evil.example/photo",
		"https://lh3.googleusercontent.com:444/photo",
		"https://user@lh3.googleusercontent.com/photo",
	} {
		if got := validGooglePictureURL(value); got != "" {
			t.Errorf("validGooglePictureURL(%q) = %q, want empty", value, got)
		}
	}
}

func TestFindOrCreateGoogleUserLinksVerifiedEmailToExistingAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`WHERE u\.google_subject = \$1`).WithArgs("google-subject").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`WHERE lower\(u\.email\) = lower\(\$1\)`).WithArgs("owner@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "shop_id", "full_name", "email", "role", "shop_name", "currency", "google_picture_url", "active"}).
			AddRow("user-id", "shop-id", "Existing Owner", "owner@example.com", "owner", "Existing Shop", "UGX", "", true))
	mock.ExpectQuery(`SELECT google_subject FROM users WHERE id = \$1`).WithArgs("user-id").
		WillReturnRows(sqlmock.NewRows([]string{"google_subject"}).AddRow(nil))
	const pictureURL = "https://lh3.googleusercontent.com/a/linked-photo"
	mock.ExpectExec(`UPDATE users SET google_subject = \$1, email_verified = TRUE,.*WHERE id = \$2 AND google_subject IS NULL`).
		WithArgs("google-subject", "user-id", pictureURL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	server := New(db, Config{})
	user, err := server.findOrCreateGoogleUser(context.Background(), "google-subject", googleIdentityClaims{
		Email: "owner@example.com", EmailVerified: true, Name: "Google Owner", Picture: pictureURL,
	}, "")
	if err != nil {
		t.Fatalf("link Google to existing account: %v", err)
	}
	if user.UserID != "user-id" || user.ShopName != "Existing Shop" || user.FullName != "Existing Owner" ||
		user.GooglePictureURL != pictureURL {
		t.Fatalf("existing account was not preserved: %+v", user)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestFindOrCreateGoogleUserRejectsDifferentLinkedIdentity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`WHERE u\.google_subject = \$1`).WithArgs("new-google-subject").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`WHERE lower\(u\.email\) = lower\(\$1\)`).WithArgs("owner@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "shop_id", "full_name", "email", "role", "shop_name", "currency", "google_picture_url", "active"}).
			AddRow("user-id", "shop-id", "Existing Owner", "owner@example.com", "owner", "Existing Shop", "UGX", "", true))
	mock.ExpectQuery(`SELECT google_subject FROM users WHERE id = \$1`).WithArgs("user-id").
		WillReturnRows(sqlmock.NewRows([]string{"google_subject"}).AddRow("other-google-subject"))
	mock.ExpectRollback()

	server := New(db, Config{})
	_, err = server.findOrCreateGoogleUser(context.Background(), "new-google-subject", googleIdentityClaims{
		Email: "owner@example.com", EmailVerified: true, Name: "Owner",
	}, "")
	if !errors.Is(err, errGoogleAccountConflict) {
		t.Fatalf("error = %v, want Google account conflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestLinkGoogleToExistingUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id::text FROM users WHERE google_subject = \$1 AND id != \$2`).
		WithArgs("google-sub-123", "user-uuid-1").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`UPDATE users SET google_subject = \$1 WHERE id = \$2`).
		WithArgs("google-sub-123", "user-uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT u\.id::text.*COALESCE\(u\.google_picture_url, ''\), u\.active`).
		WithArgs("user-uuid-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "shop_id", "full_name", "email", "role", "name", "currency", "google_picture_url", "active"}).
			AddRow("user-uuid-1", "shop-uuid-1", "Admin User", "admin@store.com", "owner", "Admin Shop", "UGX", "", true))
	mock.ExpectCommit()

	server := New(db, Config{})
	user, err := server.linkGoogleToExistingUser(context.Background(), "user-uuid-1", "google-sub-123", "")
	if err != nil {
		t.Fatalf("linkGoogleToExistingUser failed: %v", err)
	}
	if user.UserID != "user-uuid-1" || user.ShopName != "Admin Shop" {
		t.Fatalf("unexpected linked user: %+v", user)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations not met: %v", err)
	}
}

func googleTestConfig() Config {
	return Config{
		JWTSecret:          []byte("01234567890123456789012345678901"),
		CookieSecure:       true,
		GoogleClientID:     "test-client.apps.googleusercontent.com",
		GoogleClientSecret: "test-client-secret",
		GoogleRedirectURL:  "https://retail.example.com/api/v1/auth/google/callback",
		AppBaseURL:         "https://retail.example.com",
	}
}
