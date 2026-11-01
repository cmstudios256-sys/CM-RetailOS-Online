package config

import (
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL        string
	JWTSecret          []byte
	HTTPAddr           string
	CookieSecure       bool
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	AppBaseURL         string
	SMTPHost           string
	SMTPPort           string
	SMTPUsername       string
	SMTPPassword       string
	SMTPFrom           string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:          []byte(os.Getenv("JWT_SECRET")),
		HTTPAddr:           strings.TrimSpace(os.Getenv("HTTP_ADDR")),
		CookieSecure:       os.Getenv("COOKIE_SECURE") != "false",
		GoogleClientID:     strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
		GoogleRedirectURL:  strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URL")),
		AppBaseURL:         strings.TrimSpace(os.Getenv("APP_BASE_URL")),
		SMTPHost:           strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:           strings.TrimSpace(os.Getenv("SMTP_PORT")),
		SMTPUsername:       strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		SMTPPassword:       os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:           strings.TrimSpace(os.Getenv("SMTP_FROM")),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 || strings.Contains(string(cfg.JWTSecret), "replace-this") {
		return Config{}, fmt.Errorf("JWT_SECRET must be a non-placeholder value of at least 32 bytes")
	}
	if (cfg.GoogleClientID == "") != (cfg.GoogleClientSecret == "") {
		return Config{}, fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must both be configured")
	}
	smtpValues := []string{cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom}
	configuredSMTPValues := 0
	for _, value := range smtpValues {
		if value != "" {
			configuredSMTPValues++
		}
	}
	if configuredSMTPValues != 0 && configuredSMTPValues != len(smtpValues) {
		return Config{}, fmt.Errorf("SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, and SMTP_FROM must all be configured together")
	}
	if configuredSMTPValues == len(smtpValues) {
		port, err := strconv.Atoi(cfg.SMTPPort)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("SMTP_PORT must be between 1 and 65535")
		}
		if _, err := mail.ParseAddress(cfg.SMTPFrom); err != nil {
			return Config{}, fmt.Errorf("SMTP_FROM must be a valid email address")
		}
		if cfg.AppBaseURL == "" {
			return Config{}, fmt.Errorf("APP_BASE_URL is required when SMTP password reset is configured")
		}
		if _, err := parseOAuthURL("APP_BASE_URL", cfg.AppBaseURL); err != nil {
			return Config{}, err
		}
	}
	if cfg.GoogleClientID != "" {
		redirectURL, err := parseOAuthURL("GOOGLE_REDIRECT_URL", cfg.GoogleRedirectURL)
		if err != nil {
			return Config{}, err
		}
		appURL, err := parseOAuthURL("APP_BASE_URL", cfg.AppBaseURL)
		if err != nil {
			return Config{}, err
		}
		if redirectURL.Hostname() != appURL.Hostname() {
			return Config{}, fmt.Errorf("GOOGLE_REDIRECT_URL and APP_BASE_URL must use the same hostname for session cookies")
		}
		if redirectURL.Path != "/api/v1/auth/google/callback" || redirectURL.RawQuery != "" || redirectURL.Fragment != "" {
			return Config{}, fmt.Errorf("GOOGLE_REDIRECT_URL must end in /api/v1/auth/google/callback without a query or fragment")
		}
		if appURL.Path != "" && appURL.Path != "/" || appURL.RawQuery != "" || appURL.Fragment != "" {
			return Config{}, fmt.Errorf("APP_BASE_URL must be the application origin without a path, query, or fragment")
		}
	}
	return cfg, nil
}

func parseOAuthURL(name, value string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" && parsed.Scheme != "http" || parsed.User != nil {
		return nil, fmt.Errorf("%s must be an absolute HTTP or HTTPS URL", name)
	}
	if parsed.Scheme == "http" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1" {
		return nil, fmt.Errorf("%s must use HTTPS outside local development", name)
	}
	return parsed, nil
}
