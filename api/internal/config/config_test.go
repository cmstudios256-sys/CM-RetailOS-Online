package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	setGoogleEnvironment(t, "", "", "", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an empty DATABASE_URL")
	}
}

func TestLoadRejectsPlaceholderSecret(t *testing.T) {
	setGoogleEnvironment(t, "", "", "", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "replace-this-with-a-random-secret-at-least-32-bytes-long")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted the example JWT_SECRET")
	}
}

func TestLoadRejectsInsecureCookieInProduction(t *testing.T) {
	setGoogleEnvironment(t, "client-id", "client-secret", "https://example.com/api/v1/auth/google/callback", "https://example.com")
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted insecure cookies in production")
	}
}

func TestLoadRequiresGoogleInProduction(t *testing.T) {
	setGoogleEnvironment(t, "", "", "", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted production mode without Google OAuth configuration")
	}
}

func TestLoadUsesSecureCookieByDefault(t *testing.T) {
	setGoogleEnvironment(t, "", "", "", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("COOKIE_SECURE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.CookieSecure {
		t.Fatal("CookieSecure should default to true")
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
}

func TestLoadRequiresGoogleOAuthCredentialsTogether(t *testing.T) {
	setGoogleEnvironment(t, "client-id.apps.googleusercontent.com", "", "", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted only one Google OAuth credential")
	}
}

func TestLoadRequiresValidGoogleOAuthURLs(t *testing.T) {
	setGoogleEnvironment(t, "client-id.apps.googleusercontent.com", "client-secret", "", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted Google OAuth credentials without callback URLs")
	}
}

func TestLoadAcceptsLocalGoogleOAuthURLs(t *testing.T) {
	setGoogleEnvironment(
		t,
		"client-id.apps.googleusercontent.com",
		"client-secret",
		"http://localhost:8080/api/v1/auth/google/callback",
		"http://localhost:5180",
	)
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("COOKIE_SECURE", "false")

	if _, err := Load(); err != nil {
		t.Fatalf("Load() rejected local Google OAuth URLs: %v", err)
	}
}

func TestLoadRejectsGoogleOAuthURLWithDifferentHostname(t *testing.T) {
	setGoogleEnvironment(
		t,
		"client-id.apps.googleusercontent.com",
		"client-secret",
		"https://api.example.com/api/v1/auth/google/callback",
		"https://app.example.com",
	)
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted Google OAuth URLs on different hostnames")
	}
}

func TestLoadAcceptsSMTPPasswordResetConfiguration(t *testing.T) {
	setGoogleEnvironment(t, "", "", "", "http://localhost:5180")
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "retail@example.com")
	t.Setenv("SMTP_PASSWORD", "app-password")
	t.Setenv("SMTP_FROM", "retail@example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() rejected SMTP configuration: %v", err)
	}
	if cfg.SMTPHost != "smtp.example.com" || cfg.SMTPPort != "587" {
		t.Fatalf("SMTP config = %q:%q, want smtp.example.com:587", cfg.SMTPHost, cfg.SMTPPort)
	}
}

func TestLoadRejectsPartialSMTPConfiguration(t *testing.T) {
	setGoogleEnvironment(t, "", "", "", "http://localhost:5180")
	t.Setenv("DATABASE_URL", "postgres://localhost/retailos")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("SMTP_HOST", "smtp.example.com")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted partial SMTP settings")
	}
}

func setGoogleEnvironment(t *testing.T, clientID, clientSecret, redirectURL, appBaseURL string) {
	t.Helper()
	t.Setenv("GOOGLE_CLIENT_ID", clientID)
	t.Setenv("GOOGLE_CLIENT_SECRET", clientSecret)
	t.Setenv("GOOGLE_REDIRECT_URL", redirectURL)
	t.Setenv("APP_BASE_URL", appBaseURL)
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("SMTP_FROM", "")
}
