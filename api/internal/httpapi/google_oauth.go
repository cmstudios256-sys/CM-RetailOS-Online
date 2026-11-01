package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

const (
	googleIssuer       = "https://accounts.google.com"
	googleStateCookie  = "cm_retailos_google_state"
	googleNonceCookie  = "cm_retailos_google_nonce"
	googlePKCECookie   = "cm_retailos_google_pkce"
	googleFlowDuration = 10 * time.Minute
	googleCallbackPath = "/api/v1/auth/google/callback"
	googleStateIssuer  = "cm-retailos-online-google"
)

type googleStateClaims struct {
	Nonce      string `json:"nonce"`
	ShopName   string `json:"shop_name,omitempty"`
	Verifier   string `json:"verifier,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	ShopID     string `json:"shop_id,omitempty"`
	NextPage   string `json:"next_page,omitempty"`
	ClaimGuest bool   `json:"claim_guest,omitempty"`
	jwt.RegisteredClaims
}

type googleIdentityClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (s *Server) googleOAuthEnabled() bool {
	return s.config.GoogleClientID != "" && s.config.GoogleClientSecret != "" &&
		s.config.GoogleRedirectURL != "" && s.config.AppBaseURL != ""
}

func (s *Server) authProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"googleEnabled":            s.googleOAuthEnabled(),
		"emailEnabled":             true,
		"emailVerificationEnabled": s.emailVerificationEnabled(),
		"passwordResetEnabled":     s.passwordResetEnabled(),
	})
}

func (s *Server) googleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.config.GoogleClientID,
		ClientSecret: s.config.GoogleClientSecret,
		RedirectURL:  s.config.GoogleRedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:  "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

func (s *Server) googleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.googleOAuthEnabled() {
		writeError(w, http.StatusServiceUnavailable, "google_sign_in_unavailable", "Google sign-in is not configured yet.")
		return
	}

	shopName := strings.TrimSpace(r.URL.Query().Get("shopName"))
	if len([]rune(shopName)) > 120 || strings.ContainsRune(shopName, '\x00') {
		writeError(w, http.StatusBadRequest, "invalid_shop_name", "Shop name must be 120 characters or fewer.")
		return
	}
	nextPage := validWorkspacePage(r.URL.Query().Get("next"))

	nonce, err := randomOAuthValue(32)
	if err != nil {
		log.Printf("create Google sign-in nonce: %v", err)
		writeError(w, http.StatusInternalServerError, "google_sign_in_failed", "Google sign-in could not be started.")
		return
	}
	verifier, err := randomOAuthValue(32)
	if err != nil {
		log.Printf("create Google sign-in PKCE verifier: %v", err)
		writeError(w, http.StatusInternalServerError, "google_sign_in_failed", "Google sign-in could not be started.")
		return
	}
	var existingUserID, existingShopID string
	if authClaims := claimsFrom(r); authClaims != nil {
		existingUserID = authClaims.Subject
		existingShopID = authClaims.ShopID
	}

	now := time.Now()
	stateToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, googleStateClaims{
		Nonce:      nonce,
		ShopName:   shopName,
		Verifier:   verifier,
		UserID:     existingUserID,
		ShopID:     existingShopID,
		NextPage:   nextPage,
		ClaimGuest: r.URL.Query().Get("claim") == "guest",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    googleStateIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(googleFlowDuration)),
		},
	}).SignedString(s.config.JWTSecret)
	if err != nil {
		log.Printf("sign Google sign-in state: %v", err)
		writeError(w, http.StatusInternalServerError, "google_sign_in_failed", "Google sign-in could not be started.")
		return
	}

	for name, value := range map[string]string{
		googleStateCookie: stateToken,
		googleNonceCookie: nonce,
		googlePKCECookie:  verifier,
	} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: value, Path: googleCallbackPath, HttpOnly: true,
			Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode,
			Expires: now.Add(googleFlowDuration), MaxAge: int(googleFlowDuration.Seconds()),
		})
	}
	http.Redirect(w, r, s.googleOAuthConfig().AuthCodeURL(
		stateToken,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	), http.StatusFound)
}

func (s *Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, stateErr := r.Cookie(googleStateCookie)
	nonceCookie, nonceErr := r.Cookie(googleNonceCookie)
	pkceCookie, pkceErr := r.Cookie(googlePKCECookie)
	clearGoogleFlowCookies(w, s.config.CookieSecure)

	state := r.URL.Query().Get("state")
	if state == "" {
		s.redirectGoogleError(w, r, "google_sign_in_expired")
		return
	}

	claims := &googleStateClaims{}
	stateToken, err := jwt.ParseWithClaims(state, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected Google OAuth state signing method")
		}
		return s.config.JWTSecret, nil
	}, jwt.WithIssuer(googleStateIssuer), jwt.WithExpirationRequired())
	if err != nil || !stateToken.Valid {
		s.redirectGoogleError(w, r, "google_sign_in_expired")
		return
	}

	if stateErr == nil && stateCookie != nil && subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		s.redirectGoogleError(w, r, "google_sign_in_expired")
		return
	}
	if nonceErr == nil && nonceCookie != nil && subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(nonceCookie.Value)) != 1 {
		s.redirectGoogleError(w, r, "google_sign_in_expired")
		return
	}

	pkceVerifier := claims.Verifier
	if pkceErr == nil && pkceCookie != nil && pkceCookie.Value != "" {
		pkceVerifier = pkceCookie.Value
	}
	if pkceVerifier == "" {
		s.redirectGoogleError(w, r, "google_sign_in_expired")
		return
	}

	if providerError := r.URL.Query().Get("error"); providerError != "" {
		if providerError == "access_denied" {
			s.redirectGoogleError(w, r, "google_sign_in_cancelled")
		} else {
			s.redirectGoogleError(w, r, "google_sign_in_failed")
		}
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	token, err := s.googleOAuthConfig().Exchange(ctx, code, oauth2.VerifierOption(pkceVerifier))
	if err != nil {
		log.Printf("exchange Google authorization code: %v", err)
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}
	idTokenValue, ok := token.Extra("id_token").(string)
	if !ok || idTokenValue == "" {
		log.Printf("Google token response did not contain an ID token")
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}
	verifier, err := s.googleIDTokenVerifier(ctx)
	if err != nil {
		log.Printf("initialize Google ID token verification: %v", err)
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}
	idToken, err := verifier.Verify(ctx, idTokenValue)
	if err != nil {
		log.Printf("verify Google ID token: %v", err)
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(claims.Nonce)) != 1 {
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}

	var profile googleIdentityClaims
	if err := idToken.Claims(&profile); err != nil {
		log.Printf("decode verified Google identity: %v", err)
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))
	profile.Picture = validGooglePictureURL(profile.Picture)
	parsedEmail, err := mail.ParseAddress(profile.Email)
	if err != nil || parsedEmail.Address != profile.Email || !profile.EmailVerified || idToken.Subject == "" {
		s.redirectGoogleError(w, r, "google_email_unverified")
		return
	}

	// If linking to an existing authenticated user
	if claims.UserID != "" && claims.ShopID != "" {
		user, err := s.linkGoogleToExistingUser(ctx, claims.UserID, idToken.Subject, profile.Picture)
		if err != nil {
			log.Printf("link Google identity to existing account: %v", err)
			s.redirectGoogleError(w, r, "google_account_conflict")
			return
		}
		if err := s.setSession(w, user); err != nil {
			log.Printf("create session after Google account link: %v", err)
			s.redirectGoogleError(w, r, "google_sign_in_failed")
			return
		}
		destination, err := s.googleCallbackDestination(claims.NextPage, claims.ClaimGuest)
		if err != nil {
			log.Printf("build Google link redirect: %v", err)
			s.redirectGoogleError(w, r, "google_sign_in_failed")
			return
		}
		query := destination.Query()
		query.Set("google_linked", profile.Email)
		destination.RawQuery = query.Encode()
		http.Redirect(w, r, destination.String(), http.StatusSeeOther)
		return
	}

	user, err := s.findOrCreateGoogleUser(ctx, idToken.Subject, profile, claims.ShopName)
	if err != nil {
		if errors.Is(err, errGoogleAccountConflict) {
			s.redirectGoogleError(w, r, "google_account_conflict")
			return
		}
		if errors.Is(err, errGoogleAccountDisabled) {
			s.redirectGoogleError(w, r, "google_account_disabled")
			return
		}
		log.Printf("load or create account for verified Google identity: %v", err)
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}
	if err := s.setSession(w, user); err != nil {
		log.Printf("create session after Google sign-in: %v", err)
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}
	destination, err := s.googleCallbackDestination(claims.NextPage, claims.ClaimGuest)
	if err != nil {
		log.Printf("build Google sign-in redirect: %v", err)
		s.redirectGoogleError(w, r, "google_sign_in_failed")
		return
	}
	http.Redirect(w, r, destination.String(), http.StatusSeeOther)
}

func (s *Server) googleCallbackDestination(nextPage string, claimGuest bool) (*url.URL, error) {
	destination, err := url.Parse(s.config.AppBaseURL)
	if err != nil || destination.Scheme == "" || destination.Host == "" {
		return nil, fmt.Errorf("invalid app base URL for Google callback redirect")
	}
	if nextPage = validWorkspacePage(nextPage); nextPage != "" && nextPage != "dashboard" {
		query := destination.Query()
		query.Set("page", nextPage)
		destination.RawQuery = query.Encode()
	}
	if claimGuest {
		query := destination.Query()
		query.Set("claim", "guest")
		destination.RawQuery = query.Encode()
	}
	return destination, nil
}

func validWorkspacePage(page string) string {
	switch page {
	case "dashboard", "pos", "sales", "receipts", "profit-analytics", "business-intelligence", "products", "stock-levels", "audit-logs", "settings":
		return page
	default:
		return ""
	}
}

func (s *Server) googleIDTokenVerifier(ctx context.Context) (*oidc.IDTokenVerifier, error) {
	s.googleProviderMu.Lock()
	defer s.googleProviderMu.Unlock()
	if s.googleProvider == nil {
		provider, err := oidc.NewProvider(ctx, googleIssuer)
		if err != nil {
			return nil, fmt.Errorf("discover Google OpenID Connect provider: %w", err)
		}
		s.googleProvider = provider
	}
	return s.googleProvider.Verifier(&oidc.Config{ClientID: s.config.GoogleClientID}), nil
}

var (
	errGoogleAccountConflict = errors.New("Google identity conflicts with an existing account")
	errGoogleAccountDisabled = errors.New("Google account is disabled")
)

func (s *Server) linkGoogleToExistingUser(ctx context.Context, userID, subject, pictureURL string) (identity, error) {
	pictureURL = validGooglePictureURL(pictureURL)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identity{}, fmt.Errorf("begin Google account link transaction: %w", err)
	}
	defer tx.Rollback()

	var conflictingUserID string
	err = tx.QueryRowContext(ctx, `SELECT id::text FROM users WHERE google_subject = $1 AND id != $2`, subject, userID).Scan(&conflictingUserID)
	if err == nil {
		return identity{}, errGoogleAccountConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return identity{}, fmt.Errorf("check existing Google subject: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE users SET google_subject = $1 WHERE id = $2`, subject, userID)
	if err != nil {
		return identity{}, fmt.Errorf("link Google subject: %w", err)
	}

	user, active, err := readGoogleUser(ctx, tx, `u.id = $1`, userID)
	if err != nil {
		return identity{}, fmt.Errorf("read linked user: %w", err)
	}
	if !active {
		return identity{}, errGoogleAccountDisabled
	}
	if err := updateGooglePicture(ctx, tx, userID, pictureURL); err != nil {
		return identity{}, fmt.Errorf("save linked Google profile picture: %w", err)
	}
	if pictureURL != "" {
		user.GooglePictureURL = pictureURL
	}

	if err := tx.Commit(); err != nil {
		return identity{}, fmt.Errorf("commit Google account link: %w", err)
	}
	user.GoogleConnected = true
	user.GoogleEmail = user.Email
	return user, nil
}

func (s *Server) findOrCreateGoogleUser(ctx context.Context, subject string, profile googleIdentityClaims, shopName string) (identity, error) {
	profile.Picture = validGooglePictureURL(profile.Picture)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identity{}, fmt.Errorf("begin Google account transaction: %w", err)
	}
	defer tx.Rollback()

	user, active, err := readGoogleUser(ctx, tx, `u.google_subject = $1`, subject)
	if err == nil {
		if !active {
			return identity{}, errGoogleAccountDisabled
		}
		if err := updateGooglePicture(ctx, tx, user.UserID, profile.Picture); err != nil {
			return identity{}, fmt.Errorf("refresh Google profile picture: %w", err)
		}
		if profile.Picture != "" {
			user.GooglePictureURL = profile.Picture
		}
		if err := tx.Commit(); err != nil {
			return identity{}, fmt.Errorf("commit Google sign-in: %w", err)
		}
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return identity{}, fmt.Errorf("find Google account by subject: %w", err)
	}

	user, active, err = readGoogleUser(ctx, tx, `lower(u.email) = lower($1)`, profile.Email)
	if err == nil {
		if !active {
			return identity{}, errGoogleAccountDisabled
		}
		var existingSubject sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT google_subject FROM users WHERE id = $1`, user.UserID).Scan(&existingSubject); err != nil {
			return identity{}, fmt.Errorf("read existing Google account link: %w", err)
		}
		if existingSubject.Valid && existingSubject.String != subject {
			return identity{}, errGoogleAccountConflict
		}
		if !existingSubject.Valid {
			result, err := tx.ExecContext(ctx, `
				UPDATE users SET google_subject = $1, email_verified = TRUE,
					google_picture_url = COALESCE(NULLIF($3, ''), google_picture_url)
				WHERE id = $2 AND google_subject IS NULL`, subject, user.UserID, profile.Picture)
			if err != nil {
				if isUniqueViolation(err) {
					return identity{}, errGoogleAccountConflict
				}
				return identity{}, fmt.Errorf("link Google identity to existing account: %w", err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return identity{}, fmt.Errorf("check Google account link result: %w", err)
			}
			if affected != 1 {
				return identity{}, errGoogleAccountConflict
			}
		}
		if profile.Picture != "" {
			user.GooglePictureURL = profile.Picture
		}
		if err := tx.Commit(); err != nil {
			return identity{}, fmt.Errorf("commit Google account link: %w", err)
		}
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return identity{}, fmt.Errorf("find Google account by email: %w", err)
	}

	shopName = strings.TrimSpace(shopName)
	fullName := "Shop Owner"
	if shopName == "" {
		shopName = "My Shop"
	}
	if runes := []rune(shopName); len(runes) > 120 {
		shopName = string(runes[:120])
	}
	if !validLength(shopName, 1, 120) || strings.ContainsRune(shopName, '\x00') {
		return identity{}, errors.New("invalid shop name")
	}
	var shopID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO shops (name) VALUES ($1) RETURNING id::text`, shopName).Scan(&shopID); err != nil {
		return identity{}, fmt.Errorf("create shop for Google account: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (shop_id, full_name, email, google_subject, google_picture_url, role)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), 'owner')
		RETURNING id::text, shop_id::text, full_name, email, role`,
		shopID, fullName, profile.Email, subject, profile.Picture,
	).Scan(&user.UserID, &user.ShopID, &user.FullName, &user.Email, &user.Role); err != nil {
		if isUniqueViolation(err) {
			return identity{}, errGoogleAccountConflict
		}
		return identity{}, fmt.Errorf("create owner account for Google identity: %w", err)
	}
	user.ShopName = shopName
	user.Currency = "UGX"
	user.GoogleConnected = true
	user.GoogleEmail = profile.Email
	user.GooglePictureURL = profile.Picture
	if err := tx.Commit(); err != nil {
		if isUniqueViolation(err) {
			return identity{}, errGoogleAccountConflict
		}
		return identity{}, fmt.Errorf("commit new Google owner account: %w", err)
	}
	return user, nil
}

func readGoogleUser(ctx context.Context, tx *sql.Tx, predicate string, value string) (identity, bool, error) {
	var user identity
	var active bool
	err := tx.QueryRowContext(ctx, `
		SELECT u.id::text, u.shop_id::text, u.full_name, u.email, u.role, sh.name, sh.currency,
		       COALESCE(u.google_picture_url, ''), u.active
		FROM users u JOIN shops sh ON sh.id = u.shop_id
		WHERE `+predicate, value,
	).Scan(&user.UserID, &user.ShopID, &user.FullName, &user.Email, &user.Role, &user.ShopName, &user.Currency, &user.GooglePictureURL, &active)
	if err == nil {
		user.GooglePictureURL = validGooglePictureURL(user.GooglePictureURL)
		user.GoogleConnected = true
		user.GoogleEmail = user.Email
	}
	return user, active, err
}

func updateGooglePicture(ctx context.Context, tx *sql.Tx, userID, pictureURL string) error {
	if pictureURL == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET google_picture_url = $1 WHERE id = $2`, pictureURL, userID); err != nil {
		return err
	}
	return nil
}

func validGooglePictureURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.Port() != "" || parsed.User != nil ||
		parsed.Fragment != "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "googleusercontent.com" && !strings.HasSuffix(host, ".googleusercontent.com") {
		return ""
	}
	return parsed.String()
}

func (s *Server) redirectGoogleError(w http.ResponseWriter, r *http.Request, code string) {
	destination, err := url.Parse(s.config.AppBaseURL)
	if err != nil || destination.Scheme == "" || destination.Host == "" {
		writeError(w, http.StatusBadGateway, "google_sign_in_failed", "Google sign-in could not be completed.")
		return
	}
	query := destination.Query()
	query.Set("google_error", code)
	destination.RawQuery = query.Encode()
	http.Redirect(w, r, destination.String(), http.StatusSeeOther)
}

func clearGoogleFlowCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{googleStateCookie, googleNonceCookie, googlePKCECookie} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: googleCallbackPath, HttpOnly: true,
			Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
		})
	}
}

func randomOAuthValue(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate OAuth value: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
