package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func TestEmailAndPasswordValidation(t *testing.T) {
	for _, email := range []string{"owner@example.com", "first.last+shop@example.co.ug"} {
		if !validEmail(email) {
			t.Errorf("validEmail(%q) = false", email)
		}
	}
	for _, email := range []string{"", "not-an-email", "Owner <owner@example.com>", "owner@example.com\r\nBcc:attacker@example.com"} {
		if validEmail(email) {
			t.Errorf("validEmail(%q) = true, want false", email)
		}
	}
	if !validPassword("long-password-123") {
		t.Fatal("validPassword rejected a password meeting the minimum length")
	}
	if !validPassword("12345678") {
		t.Fatal("validPassword rejected an 8-character password")
	}
	if validPassword("1234567") {
		t.Fatal("validPassword accepted a 7-character password")
	}
	if validPassword("short") {
		t.Fatal("validPassword accepted a short password")
	}
	if validPassword(strings.Repeat("x", maximumPasswordBytes+1)) {
		t.Fatal("validPassword accepted a password exceeding bcrypt's byte limit")
	}
}

func TestEmailVerificationRequiresCompleteSMTPConfiguration(t *testing.T) {
	config := Config{
		AppBaseURL:   "https://retail.example.com",
		SMTPHost:     "smtp-relay.brevo.com",
		SMTPPort:     "587",
		SMTPUsername: "brevo-login",
		SMTPPassword: "smtp-key",
		SMTPFrom:     "retail@example.com",
	}
	if !New(nil, config).emailVerificationEnabled() {
		t.Fatal("email verification disabled with complete SMTP configuration")
	}
	config.SMTPPassword = ""
	if New(nil, config).emailVerificationEnabled() {
		t.Fatal("email verification enabled with incomplete SMTP configuration")
	}
}

func TestUnverifiedEmailCannotSignIn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}
	mock.ExpectQuery(`(?s)SELECT u\.id::text.*u\.email_verified.*FROM users u`).
		WithArgs("owner@example.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "shop_id", "full_name", "email", "role", "password_hash", "email_verified",
			"shop_name", "currency", "google_picture_url", "google_subject", "cloud_gmail",
		}).AddRow("user-id", "shop-id", "Owner", "owner@example.com", "owner", passwordHash, false, "Corner Shop", "UGX", nil, nil, nil))
	server := New(db, Config{JWTSecret: []byte("01234567890123456789012345678901")})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"owner@example.com","password":"correct-password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "email_not_verified") {
		t.Fatalf("response = %d %s, want email verification required", response.Code, response.Body.String())
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("unverified account unexpectedly received a session cookie")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("login expectations: %v", err)
	}
}

func TestGoogleOnlyAccountExplainsPasswordSignInOptions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()
	mock.ExpectQuery(`(?s)SELECT u\.id::text.*u\.email_verified.*FROM users u`).
		WithArgs("owner@example.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "shop_id", "full_name", "email", "role", "password_hash", "email_verified",
			"shop_name", "currency", "google_picture_url", "google_subject", "cloud_gmail",
		}).AddRow("user-id", "shop-id", "Owner", "owner@example.com", "owner", nil, true, "Corner Shop", "UGX", nil, "google-subject", nil))
	server := New(db, Config{JWTSecret: []byte("01234567890123456789012345678901")})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"owner@example.com","password":"google-account-password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "google_password_not_set") ||
		!strings.Contains(response.Body.String(), "Sign in with Google") {
		t.Fatalf("response = %d %s, want Google sign-in guidance", response.Code, response.Body.String())
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("Google-only account unexpectedly received a session cookie")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("login expectations: %v", err)
	}
}

func TestConfirmEmailVerificationConsumesToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	token := "single-use-email-verification-token"
	tokenHash := sha256.Sum256([]byte(token))
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)DELETE FROM email_verification_tokens.*RETURNING user_id::text`).
		WithArgs(tokenHash[:]).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("user-id"))
	mock.ExpectExec(`UPDATE users SET email_verified = TRUE.*WHERE id = \$1 AND active = TRUE AND email_verified = FALSE`).
		WithArgs("user-id").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	server := New(db, Config{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/email-verification/confirm",
		strings.NewReader(`{"token":"single-use-email-verification-token"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Email verified") {
		t.Fatalf("response = %d %s, want email verification success", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("email verification expectations: %v", err)
	}
}

func TestPasswordResetRequestExplainsMissingSMTPConfiguration(t *testing.T) {
	server := New(nil, Config{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request",
		strings.NewReader(`{"email":"owner@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "password_reset_unavailable") {
		t.Fatalf("response = %d %s, want clear SMTP setup error", response.Code, response.Body.String())
	}
}

func TestPasswordSetupRequestIncludesVerifiedGoogleAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()
	mock.ExpectQuery(`(?s)SELECT id::text FROM users.*email_verified = TRUE.*password_hash IS NOT NULL OR google_subject IS NOT NULL`).
		WithArgs("owner@example.com").
		WillReturnError(sql.ErrNoRows)
	server := New(db, Config{
		AppBaseURL:   "https://retail.example.com",
		SMTPHost:     "smtp-relay.brevo.com",
		SMTPPort:     "587",
		SMTPUsername: "brevo-login",
		SMTPPassword: "smtp-key",
		SMTPFrom:     "retail@example.com",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request",
		strings.NewReader(`{"email":"owner@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "reset link will be sent") {
		t.Fatalf("response = %d %s, want generic password link response", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("password setup request expectations: %v", err)
	}
}

func TestConfirmPasswordResetConsumesToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	token := "single-use-reset-token"
	tokenHash := sha256.Sum256([]byte(token))
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)DELETE FROM password_reset_tokens.*RETURNING user_id::text`).
		WithArgs(tokenHash[:]).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("user-id"))
	mock.ExpectExec(`UPDATE users SET password_hash = \$1 WHERE id = \$2 AND active = TRUE`).
		WithArgs(sqlmock.AnyArg(), "user-id").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	server := New(db, Config{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm",
		strings.NewReader(`{"token":"single-use-reset-token","password":"new-long-password-123"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Password updated") {
		t.Fatalf("response = %d %s, want password reset success", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("password reset expectations: %v", err)
	}
}
