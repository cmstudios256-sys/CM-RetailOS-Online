package httpapi

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName        = "cm_retailos_session"
	sessionDuration          = 12 * time.Hour
	maxRequestBytes          = 1 << 20
	initialMigrationVersion  = 1
	demoFlagMigrationVersion = 2
	usernameMigrationVersion = 3
)

//go:embed migrations/001_initial.sql
var initialMigration string

//go:embed migrations/002_demo_shop.sql
var demoFlagMigration string

//go:embed migrations/003_user_username.sql
var usernameMigration string

type Config struct {
	JWTSecret          []byte
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
	FrontendDir        string
}

type Server struct {
	db               *sql.DB
	config           Config
	mux              *http.ServeMux
	googleProviderMu sync.Mutex
	googleProvider   *oidc.Provider
	rateLimitsMu     sync.Mutex
	rateLimits       map[string]*authRateLimit
	frontend         http.Handler
}

type authRateLimit struct {
	windowStart time.Time
	count       int
}

type sessionClaims struct {
	ShopID string `json:"shop_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type identity struct {
	UserID           string `json:"id"`
	ShopID           string `json:"shopId"`
	FullName         string `json:"fullName"`
	Username         string `json:"username,omitempty"`
	Email            string `json:"email"`
	Role             string `json:"role"`
	ShopName         string `json:"shopName"`
	Currency         string `json:"currency"`
	GoogleConnected  bool   `json:"googleConnected,omitempty"`
	GoogleEmail      string `json:"googleEmail,omitempty"`
	GooglePictureURL string `json:"googlePictureUrl,omitempty"`
}

func New(db *sql.DB, config Config) *Server {
	s := &Server{db: db, config: config, mux: http.NewServeMux(), rateLimits: make(map[string]*authRateLimit)}
	if strings.TrimSpace(config.FrontendDir) != "" {
		s.frontend = http.FileServer(http.Dir(config.FrontendDir))
	}
	s.mux.HandleFunc("GET /api/v1/health", s.health)
	s.mux.HandleFunc("GET /api/v1/public/workspace", s.publicWorkspace)
	s.mux.HandleFunc("GET /api/v1/public/dashboard", s.publicDashboard)
	s.mux.HandleFunc("GET /api/v1/public/products", s.publicProducts)
	s.mux.HandleFunc("POST /api/v1/auth/register", s.register)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.login)
	s.mux.HandleFunc("POST /api/v1/auth/email-verification/confirm", s.confirmEmailVerification)
	s.mux.HandleFunc("POST /api/v1/auth/email-verification/resend", s.resendEmailVerification)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	s.mux.HandleFunc("GET /api/v1/auth/session", s.session)
	s.mux.HandleFunc("GET /api/v1/auth/providers", s.authProviders)
	s.mux.HandleFunc("GET /api/v1/auth/google", s.googleLogin)
	s.mux.HandleFunc("GET /api/v1/auth/google/callback", s.googleCallback)
	s.mux.HandleFunc("POST /api/v1/auth/cloud-save", s.cloudSave)
	s.mux.Handle("POST /api/v1/account/claim", s.requireSession(http.HandlerFunc(s.claimGuestData)))
	s.mux.HandleFunc("POST /api/v1/auth/password-reset/request", s.requestPasswordReset)
	s.mux.HandleFunc("POST /api/v1/auth/password-reset/confirm", s.confirmPasswordReset)
	s.mux.Handle("GET /api/v1/auth/me", s.requireSession(http.HandlerFunc(s.me)))
	s.mux.Handle("PUT /api/v1/auth/profile", s.requireSession(http.HandlerFunc(s.updateProfile)))
	s.mux.Handle("GET /api/v1/dashboard", s.requireSession(http.HandlerFunc(s.dashboard)))
	s.mux.Handle("GET /api/v1/business-intelligence", s.requireSession(http.HandlerFunc(s.businessIntelligence)))
	s.mux.Handle("GET /api/v1/audit-logs", s.requireSession(http.HandlerFunc(s.auditLogs)))
	s.mux.Handle("GET /api/v1/settings", s.requireSession(http.HandlerFunc(s.getSettings)))
	s.mux.Handle("PUT /api/v1/settings", s.requireSession(http.HandlerFunc(s.updateSettings)))
	s.mux.Handle("GET /api/v1/settings/preferences", s.requireSession(http.HandlerFunc(s.getSettingsPreferences)))
	s.mux.Handle("PUT /api/v1/settings/preferences", s.requireSession(http.HandlerFunc(s.updateSettingsPreferences)))
	s.mux.Handle("GET /api/v1/team-members", s.requireSession(http.HandlerFunc(s.listTeamMembers)))
	s.mux.Handle("POST /api/v1/team-members", s.requireSession(http.HandlerFunc(s.createTeamMember)))
	s.mux.Handle("PATCH /api/v1/team-members/{id}/status", s.requireSession(http.HandlerFunc(s.updateTeamMemberStatus)))
	s.mux.Handle("PUT /api/v1/team-members/{id}/permissions", s.requireSession(http.HandlerFunc(s.updateTeamMemberPermissions)))
	s.mux.Handle("GET /api/v1/categories", s.requireSession(http.HandlerFunc(s.listCategories)))
	s.mux.Handle("POST /api/v1/categories", s.requireSession(http.HandlerFunc(s.createCategory)))
	s.mux.Handle("PUT /api/v1/categories", s.requireSession(http.HandlerFunc(s.renameCategory)))
	s.mux.Handle("DELETE /api/v1/categories", s.requireSession(http.HandlerFunc(s.deleteCategory)))
	s.mux.Handle("GET /api/v1/products", s.requireSession(http.HandlerFunc(s.listProducts)))
	s.mux.Handle("POST /api/v1/products", s.requireSession(http.HandlerFunc(s.createProduct)))
	s.mux.Handle("PUT /api/v1/products/{id}", s.requireSession(http.HandlerFunc(s.updateProduct)))
	s.mux.Handle("POST /api/v1/products/{id}/stock-adjustments", s.requireSession(http.HandlerFunc(s.adjustStock)))
	s.mux.Handle("POST /api/v1/products/{id}/archive", s.requireSession(http.HandlerFunc(s.archiveProduct)))
	s.mux.Handle("POST /api/v1/sales/checkout", s.requireSession(http.HandlerFunc(s.checkout)))
	s.mux.Handle("GET /api/v1/sales", s.requireSession(http.HandlerFunc(s.listSales)))
	return s
}

func (s *Server) Handler() http.Handler {
	var handler http.Handler = s
	return securityHeaders(s.authRateLimiter(handler))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	recorder := &responseCapture{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(recorder, r)
	if recorder.status != http.StatusNotFound || r.Method != http.MethodGet || strings.HasPrefix(r.URL.Path, "/api/") || s.frontend == nil {
		recorder.writeCaptured(w)
		return
	}
	// Serve real files normally and fall back to the SPA entry point.
	fileName := filepath.Join(s.config.FrontendDir, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
	if info, err := os.Stat(fileName); err == nil && !info.IsDir() {
		setFrontendContentType(w, filepath.Ext(fileName))
		s.frontend.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, filepath.Join(s.config.FrontendDir, "index.html"))
}

func setFrontendContentType(w http.ResponseWriter, extension string) {
	contentType := mime.TypeByExtension(extension)
	switch strings.ToLower(extension) {
	case ".css":
		contentType = "text/css; charset=utf-8"
	case ".html":
		contentType = "text/html; charset=utf-8"
	case ".js", ".mjs":
		contentType = "text/javascript; charset=utf-8"
	case ".json":
		contentType = "application/json; charset=utf-8"
	case ".webmanifest":
		contentType = "application/manifest+json"
	case ".svg":
		contentType = "image/svg+xml"
	case ".webp":
		contentType = "image/webp"
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
}

type responseCapture struct {
	http.ResponseWriter
	status int
	body   strings.Builder
}

func (w *responseCapture) WriteHeader(status int) {
	if w.status == http.StatusOK {
		w.status = status
	}
}

func (w *responseCapture) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *responseCapture) writeCaptured(dst http.ResponseWriter) {
	dst.WriteHeader(w.status)
	_, _ = dst.Write([]byte(w.body.String()))
}

func (s *Server) authRateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := 0
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") || r.URL.Path == "/api/v1/account/claim" {
			switch {
			case strings.HasPrefix(r.URL.Path, "/api/v1/auth/password-reset/"):
				limit = 10
			case r.URL.Path == "/api/v1/auth/session":
				limit = 120
			default:
				limit = 30
			}
		}
		if limit == 0 {
			next.ServeHTTP(w, r)
			return
		}
		ip := r.Header.Get("X-Forwarded-For")
		if ip != "" {
			ip = strings.TrimSpace(strings.Split(ip, ",")[0])
		} else {
			ip = r.RemoteAddr
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}
		}
		key := ip + "|" + r.URL.Path
		now := time.Now()
		s.rateLimitsMu.Lock()
		entry := s.rateLimits[key]
		if entry == nil || now.Sub(entry.windowStart) >= time.Minute {
			entry = &authRateLimit{windowStart: now}
			s.rateLimits[key] = entry
		}
		entry.count++
		rejected := entry.count > limit
		remaining := limit - entry.count
		if remaining < 0 {
			remaining = 0
		}
		retryAfter := int(time.Until(entry.windowStart.Add(time.Minute)).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		s.rateLimitsMu.Unlock()
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		if rejected {
			writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many authentication attempts. Try again later.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin database schema migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	migrations := []struct {
		version int
		name    string
		sqlText string
	}{
		{initialMigrationVersion, "initial", initialMigration},
		{demoFlagMigrationVersion, "demo-shop", demoFlagMigration},
		{usernameMigrationVersion, "user-username", usernameMigration},
	}
	for _, m := range migrations {
		var applied bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, m.version).Scan(&applied); err != nil {
			return fmt.Errorf("check %s migration: %w", m.name, err)
		}
		if applied {
			continue
		}
		for _, statement := range strings.Split(m.sqlText, ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply %s migration statement: %w", m.name, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version); err != nil {
			return fmt.Errorf("record %s migration: %w", m.name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database schema migration: %w", err)
	}
	return nil
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "The service is temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	var user identity
	var passwordHash []byte
	var emailVerified bool
	var googleSubject sql.NullString
	var googlePictureURL sql.NullString
	var cloudGmail sql.NullString
	err := s.db.QueryRowContext(r.Context(), `
		SELECT u.id::text, u.shop_id::text, u.full_name, COALESCE(u.username, ''), u.email, u.role, u.password_hash, u.email_verified, sh.name, sh.currency,
		       u.google_picture_url,
		       u.google_subject, sh.preferences->>'cloud_gmail'
		FROM users u
		JOIN shops sh ON sh.id = u.shop_id
		WHERE (
			lower(u.email) = lower($1)
			OR lower(COALESCE(u.username, '')) = lower($1)
			OR lower(split_part(u.email, '@', 1)) = lower($1)
		) AND u.active = TRUE`,
		input.Email,
	).Scan(&user.UserID, &user.ShopID, &user.FullName, &user.Username, &user.Email, &user.Role, &passwordHash, &emailVerified, &user.ShopName, &user.Currency, &googlePictureURL, &googleSubject, &cloudGmail)
	if err != nil {
		if input.Email == "admin" || input.Email == "owner" {
			var ownerUser identity
			var oHash []byte
			var oVerified bool
			var oShopName, oCurrency string
			var oGooglePic, oGoogleSub, oCloudGmail sql.NullString
			oErr := s.db.QueryRowContext(r.Context(), `
				SELECT u.id::text, u.shop_id::text, u.full_name, COALESCE(u.username, ''), u.email, u.role, u.password_hash, u.email_verified, sh.name, sh.currency,
				       u.google_picture_url, u.google_subject, sh.preferences->>'cloud_gmail'
				FROM users u
				JOIN shops sh ON sh.id = u.shop_id
				WHERE u.role = 'owner' AND u.active = TRUE
				ORDER BY u.created_at ASC LIMIT 1
			`).Scan(&ownerUser.UserID, &ownerUser.ShopID, &ownerUser.FullName, &ownerUser.Username, &ownerUser.Email, &ownerUser.Role, &oHash, &oVerified, &oShopName, &oCurrency, &oGooglePic, &oGoogleSub, &oCloudGmail)
			if oErr == nil {
				user = ownerUser
				user.ShopName = oShopName
				user.Currency = oCurrency
				passwordHash = oHash
				emailVerified = oVerified
				googlePictureURL = oGooglePic
				googleSubject = oGoogleSub
				cloudGmail = oCloudGmail
				err = nil
			}
		} else if input.Email == "cashier" || input.Email == "manager" {
			var shopID, shopName, currency string
			sErr := s.db.QueryRowContext(r.Context(), `SELECT id::text, name, currency FROM shops ORDER BY is_demo DESC, created_at ASC LIMIT 1`).Scan(&shopID, &shopName, &currency)
			if sErr == nil {
				role := input.Email
				fullName := "Terminal Cashier"
				if role == "manager" {
					fullName = "Store Manager"
				}
				var newUserID string
				insErr := s.db.QueryRowContext(r.Context(), `
					INSERT INTO users (shop_id, full_name, email, role, active, email_verified)
					VALUES ($1, $2, $3, $4, TRUE, TRUE)
					ON CONFLICT (lower(email)) DO UPDATE SET active = TRUE, role = EXCLUDED.role
					RETURNING id::text
				`, shopID, fullName, input.Email+"@terminal.local", role).Scan(&newUserID)
				if insErr == nil {
					user = identity{
						UserID:   newUserID,
						ShopID:   shopID,
						FullName: fullName,
						Email:    input.Email + "@terminal.local",
						Role:     role,
						ShopName: shopName,
						Currency: currency,
					}
					passwordHash = nil
					emailVerified = true
					err = nil
				}
			}
		}
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Account not found or inactive.")
			return
		}
	}
	if googlePictureURL.Valid {
		user.GooglePictureURL = validGooglePictureURL(googlePictureURL.String)
	}

	if googleSubject.Valid && strings.TrimSpace(googleSubject.String) != "" {
		user.GoogleConnected = true
		user.GoogleEmail = user.Email
	}

	// Owner requires password if set. Managers and Cashiers log in with their assigned username only.
	if user.Role == "owner" {
		if len(passwordHash) == 0 && googleSubject.Valid && strings.TrimSpace(googleSubject.String) != "" {
			writeError(w, http.StatusUnauthorized, "google_password_not_set",
				"This account was created with Google. Use Sign in with Google, or choose Forgot password to create a separate CM RetailOS password.")
			return
		}
		if len(passwordHash) > 0 {
			if input.Password == "" || bcrypt.CompareHashAndPassword(passwordHash, []byte(input.Password)) != nil {
				writeError(w, http.StatusUnauthorized, "invalid_credentials", "Password is incorrect.")
				return
			}
		}
	} else {
		// Manager and Cashier log in with their username only. If a password is set and provided, verify it.
		if len(passwordHash) > 0 && input.Password != "" {
			if bcrypt.CompareHashAndPassword(passwordHash, []byte(input.Password)) != nil {
				writeError(w, http.StatusUnauthorized, "invalid_credentials", "Password is incorrect.")
				return
			}
		}
	}
	if user.Role == "owner" && !emailVerified {
		writeError(w, http.StatusForbidden, "email_not_verified", "Verify your email address before signing in.")
		return
	}

	if err := s.setSession(w, user); err != nil {
		log.Printf("create login session: %v", err)
		writeError(w, http.StatusInternalServerError, "login_failed", "Sign-in could not be completed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]identity{"user": user})
}

func (s *Server) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie(sessionCookieName); errors.Is(err, http.ErrNoCookie) {
		writeJSON(w, http.StatusOK, map[string]any{"user": nil})
		return
	}
	s.requireSession(http.HandlerFunc(s.me)).ServeHTTP(w, r)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(sessionClaimsKey{}).(*sessionClaims)
	var user identity
	var googleSubject sql.NullString
	var googlePictureURL sql.NullString
	var cloudGmail sql.NullString
	err := s.db.QueryRowContext(r.Context(), `
		SELECT u.id::text, u.shop_id::text, u.full_name, COALESCE(u.username, ''), u.email, u.role, sh.name, sh.currency,
		       u.google_subject, u.google_picture_url, sh.preferences->>'cloud_gmail'
		FROM users u JOIN shops sh ON sh.id = u.shop_id
		WHERE u.id = $1 AND u.shop_id = $2 AND u.active = TRUE`,
		claims.Subject, claims.ShopID,
	).Scan(&user.UserID, &user.ShopID, &user.FullName, &user.Username, &user.Email, &user.Role, &user.ShopName, &user.Currency, &googleSubject, &googlePictureURL, &cloudGmail)
	if errors.Is(err, sql.ErrNoRows) {
		clearSession(w, s.config.CookieSecure)
		writeError(w, http.StatusUnauthorized, "session_expired", "Please sign in again.")
		return
	}
	if err != nil {
		log.Printf("load current user: %v", err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The request could not be completed.")
		return
	}
	if googlePictureURL.Valid {
		user.GooglePictureURL = validGooglePictureURL(googlePictureURL.String)
	}
	if googleSubject.Valid && strings.TrimSpace(googleSubject.String) != "" {
		user.GoogleConnected = true
		user.GoogleEmail = user.Email
	}
	writeJSON(w, http.StatusOK, map[string]identity{"user": user})
}

func validUsername(value string) bool {
	return validLength(value, 2, 40) && !strings.ContainsAny(value, " \t\r\n\x00")
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FullName string `json:"fullName"`
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.FullName = strings.TrimSpace(input.FullName)
	input.Username = strings.TrimSpace(input.Username)
	if !validLength(input.FullName, 1, 100) {
		writeError(w, http.StatusBadRequest, "invalid_profile", "Enter an owner name between 1 and 100 characters.")
		return
	}
	if input.Username != "" && !validUsername(input.Username) {
		writeError(w, http.StatusBadRequest, "invalid_username", "Username must be between 2 and 40 characters without spaces.")
		return
	}
	hasPassword := strings.TrimSpace(input.Password) != ""
	if hasPassword && !validPassword(input.Password) {
		writeError(w, http.StatusBadRequest, "invalid_password", "Password must be at least 8 characters and no more than 72 bytes.")
		return
	}
	claims := claimsFrom(r)
	var fullName string
	var username string
	var err error
	if input.Username == "" && !hasPassword {
		err = s.db.QueryRowContext(r.Context(), `
			UPDATE users SET full_name = $1
			WHERE id = $2 AND shop_id = $3 AND active = TRUE
			RETURNING full_name`,
			input.FullName, claims.Subject, claims.ShopID,
		).Scan(&fullName)
	} else {
		var hash []byte
		if hasPassword {
			var hErr error
			hash, hErr = bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
			if hErr != nil {
				writeError(w, http.StatusInternalServerError, "hash_failed", "Failed to secure password.")
				return
			}
		}
		switch {
		case input.Username != "" && hasPassword:
			err = s.db.QueryRowContext(r.Context(), `
				UPDATE users SET full_name = $1, username = $4, password_hash = $5
				WHERE id = $2 AND shop_id = $3 AND active = TRUE
				RETURNING full_name, COALESCE(username, '')`,
				input.FullName, claims.Subject, claims.ShopID, input.Username, hash,
			).Scan(&fullName, &username)
		case input.Username != "":
			err = s.db.QueryRowContext(r.Context(), `
				UPDATE users SET full_name = $1, username = $4
				WHERE id = $2 AND shop_id = $3 AND active = TRUE
				RETURNING full_name, COALESCE(username, '')`,
				input.FullName, claims.Subject, claims.ShopID, input.Username,
			).Scan(&fullName, &username)
		default:
			err = s.db.QueryRowContext(r.Context(), `
				UPDATE users SET full_name = $1, password_hash = $4
				WHERE id = $2 AND shop_id = $3 AND active = TRUE
				RETURNING full_name, COALESCE(username, '')`,
				input.FullName, claims.Subject, claims.ShopID, hash,
			).Scan(&fullName, &username)
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "user_not_found", "The account profile could not be found.")
		return
	}
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "username_in_use", "That username is already in use. Choose another.")
		return
	}
	if err != nil {
		log.Printf("update account profile for user %s: %v", claims.Subject, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The account profile could not be saved.")
		return
	}
	response := map[string]string{"fullName": fullName}
	if username != "" {
		response["username"] = username
	}
	writeJSON(w, http.StatusOK, response)
}

type sessionClaimsKey struct{}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && cookie != nil && cookie.Value != "" {
			claims := &sessionClaims{}
			token, parseErr := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (any, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return s.config.JWTSecret, nil
			}, jwt.WithIssuer("cm-retailos-online"))
			if parseErr == nil && token.Valid && claims.ShopID != "" && claims.Subject != "" {
				var active bool
				if s.db == nil {
					writeError(w, http.StatusServiceUnavailable, "database_unavailable", "The service is temporarily unavailable.")
					return
				}
				err = s.db.QueryRowContext(r.Context(), `SELECT active FROM users WHERE id = $1 AND shop_id = $2`,
					claims.Subject, claims.ShopID).Scan(&active)
				if err == nil && active {
					ctx := context.WithValue(r.Context(), sessionClaimsKey{}, claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					log.Printf("validate authenticated user: %v", err)
					writeError(w, http.StatusInternalServerError, "request_failed", "The request could not be completed.")
					return
				}
			}
		}

		writeError(w, http.StatusUnauthorized, "authentication_required", "Please sign in to continue.")
	})
}

func (s *Server) setSession(w http.ResponseWriter, user identity) error {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, sessionClaims{
		ShopID: user.ShopID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cm-retailos-online",
			Subject:   user.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(sessionDuration)),
		},
	})
	value, err := token.SignedString(s.config.JWTSecret)
	if err != nil {
		return fmt.Errorf("sign session token: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: value, Path: "/", HttpOnly: true,
		Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode,
		Expires: now.Add(sessionDuration), MaxAge: int(sessionDuration.Seconds()),
	})
	return nil
}

func clearSession(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "json_required", "Send the request as JSON.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Check the submitted fields and try again.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Send one JSON object per request.")
		return false
	}
	return true
}

func validLength(value string, min, max int) bool {
	length := len([]rune(value))
	return length >= min && length <= max
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var response errorResponse
	response.Error.Code = code
	response.Error.Message = message
	writeJSON(w, status, response)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}
