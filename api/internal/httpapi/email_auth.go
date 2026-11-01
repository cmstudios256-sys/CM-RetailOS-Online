package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	minimumPasswordLength     = 8
	maximumPasswordBytes      = 72
	passwordResetDuration     = 30 * time.Minute
	emailVerificationDuration = 24 * time.Hour
)

type registerRequest struct {
	ShopName string `json:"shopName"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input registerRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ShopName = strings.TrimSpace(input.ShopName)
	input.FullName = strings.TrimSpace(input.FullName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validLength(input.ShopName, 1, 120) || strings.ContainsRune(input.ShopName, '\x00') {
		writeError(w, http.StatusBadRequest, "invalid_shop_name", "Shop name must be between 1 and 120 characters.")
		return
	}
	if !validLength(input.FullName, 1, 100) || strings.ContainsRune(input.FullName, '\x00') {
		writeError(w, http.StatusBadRequest, "invalid_full_name", "Name must be between 1 and 100 characters.")
		return
	}
	if !validEmail(input.Email) {
		writeError(w, http.StatusBadRequest, "invalid_email", "Enter a valid email address.")
		return
	}
	if !validPassword(input.Password) {
		writeError(w, http.StatusBadRequest, "invalid_password", "Password must be at least 8 characters and no more than 72 bytes.")
		return
	}
	if !s.emailVerificationEnabled() {
		writeError(w, http.StatusServiceUnavailable, "email_verification_unavailable", "Email signup is temporarily unavailable because verification email is not configured.")
		return
	}
	if s.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "The service is temporarily unavailable.")
		return
	}

	token, tokenHash, err := generateAuthToken()
	if err != nil {
		log.Printf("generate email verification token: %v", err)
		writeError(w, http.StatusInternalServerError, "registration_failed", "Account creation could not be completed.")
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("hash account password: %v", err)
		writeError(w, http.StatusInternalServerError, "registration_failed", "Account creation could not be completed.")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin account registration: %v", err)
		writeError(w, http.StatusInternalServerError, "registration_failed", "Account creation could not be completed.")
		return
	}
	defer tx.Rollback()

	var user identity
	if err := tx.QueryRowContext(r.Context(),
		`INSERT INTO shops (name) VALUES ($1) RETURNING id::text`, input.ShopName,
	).Scan(&user.ShopID); err != nil {
		log.Printf("create shop for email account: %v", err)
		writeError(w, http.StatusInternalServerError, "registration_failed", "Account creation could not be completed.")
		return
	}
	if err := tx.QueryRowContext(r.Context(), `
		INSERT INTO users (shop_id, full_name, email, password_hash, email_verified, role)
		VALUES ($1, $2, $3, $4, FALSE, 'owner') RETURNING id::text`,
		user.ShopID, input.FullName, input.Email, passwordHash,
	).Scan(&user.UserID); err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "email_in_use", "An account with this email already exists. Sign in or reset your password.")
			return
		}
		log.Printf("create email account: %v", err)
		writeError(w, http.StatusInternalServerError, "registration_failed", "Account creation could not be completed.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO email_verification_tokens (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash[:], user.UserID, time.Now().Add(emailVerificationDuration),
	); err != nil {
		log.Printf("save email verification token: %v", err)
		writeError(w, http.StatusInternalServerError, "registration_failed", "Account creation could not be completed.")
		return
	}
	user.FullName, user.Email, user.Role = input.FullName, input.Email, "owner"
	user.ShopName, user.Currency = input.ShopName, "UGX"
	if err := tx.Commit(); err != nil {
		log.Printf("commit email account registration: %v", err)
		writeError(w, http.StatusInternalServerError, "registration_failed", "Account creation could not be completed.")
		return
	}
	verificationURL := strings.TrimRight(s.config.AppBaseURL, "/") + "/?email_verification_token=" + url.QueryEscape(token)
	if err := s.sendEmailVerificationEmail(input.Email, verificationURL); err != nil {
		log.Printf("send email verification message: %v", err)
		writeError(w, http.StatusServiceUnavailable, "verification_email_failed", "Your account was created, but the verification email could not be sent. Request a new verification email to try again.")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"message": "Your account was created. Check your email for a verification link before signing in.",
	})
}

func (s *Server) emailVerificationEnabled() bool {
	return s.passwordResetEnabled()
}

func (s *Server) passwordResetEnabled() bool {
	return s.config.SMTPHost != "" && s.config.SMTPPort != "" && s.config.SMTPUsername != "" &&
		s.config.SMTPPassword != "" && s.config.SMTPFrom != "" && s.config.AppBaseURL != ""
}

type emailVerificationRequest struct {
	Token string `json:"token"`
}

func (s *Server) confirmEmailVerification(w http.ResponseWriter, r *http.Request) {
	var input emailVerificationRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Token == "" {
		writeError(w, http.StatusBadRequest, "invalid_verification_token", "Use a valid email verification link.")
		return
	}
	if s.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "The service is temporarily unavailable.")
		return
	}
	tokenHash := sha256.Sum256([]byte(input.Token))
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin email verification: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Email verification could not be completed.")
		return
	}
	defer tx.Rollback()

	var userID string
	err = tx.QueryRowContext(r.Context(), `
		DELETE FROM email_verification_tokens
		WHERE token_hash = $1 AND expires_at > now()
		RETURNING user_id::text`,
		tokenHash[:],
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "verification_link_expired", "This verification link is invalid or expired. Request a new one.")
		return
	}
	if err != nil {
		log.Printf("consume email verification token: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Email verification could not be completed.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		UPDATE users SET email_verified = TRUE
		WHERE id = $1 AND active = TRUE AND email_verified = FALSE`,
		userID,
	)
	if err != nil {
		log.Printf("mark email account verified: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Email verification could not be completed.")
		return
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			log.Printf("check email verification update: %v", err)
			writeError(w, http.StatusInternalServerError, "email_verification_failed", "Email verification could not be completed.")
		} else {
			writeError(w, http.StatusBadRequest, "verification_link_expired", "This verification link is invalid or expired. Request a new one.")
		}
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit email verification: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Email verification could not be completed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Email verified. You can now sign in."})
}

type resendEmailVerificationRequest struct {
	Email string `json:"email"`
}

func (s *Server) resendEmailVerification(w http.ResponseWriter, r *http.Request) {
	var input resendEmailVerificationRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(input.Email) {
		writeError(w, http.StatusBadRequest, "invalid_email", "Enter a valid email address.")
		return
	}
	if !s.emailVerificationEnabled() {
		writeError(w, http.StatusServiceUnavailable, "email_verification_unavailable", "Email verification is not configured yet.")
		return
	}
	if s.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "The service is temporarily unavailable.")
		return
	}
	var userID string
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id::text FROM users
		WHERE lower(email) = $1 AND active = TRUE AND password_hash IS NOT NULL AND email_verified = FALSE`,
		input.Email,
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		writeEmailVerificationRequestAccepted(w)
		return
	}
	if err != nil {
		log.Printf("find account for email verification: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Verification email could not be requested. Please try again.")
		return
	}

	token, tokenHash, err := generateAuthToken()
	if err != nil {
		log.Printf("generate replacement email verification token: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Verification email could not be requested. Please try again.")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin replacement email verification: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Verification email could not be requested. Please try again.")
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM email_verification_tokens WHERE user_id = $1`, userID); err != nil {
		log.Printf("replace email verification token: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Verification email could not be requested. Please try again.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO email_verification_tokens (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash[:], userID, time.Now().Add(emailVerificationDuration),
	); err != nil {
		log.Printf("save replacement email verification token: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Verification email could not be requested. Please try again.")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit replacement email verification: %v", err)
		writeError(w, http.StatusInternalServerError, "email_verification_failed", "Verification email could not be requested. Please try again.")
		return
	}
	verificationURL := strings.TrimRight(s.config.AppBaseURL, "/") + "/?email_verification_token=" + url.QueryEscape(token)
	if err := s.sendEmailVerificationEmail(input.Email, verificationURL); err != nil {
		log.Printf("send replacement email verification: %v", err)
		writeError(w, http.StatusServiceUnavailable, "verification_email_failed", "The verification email could not be sent. Please try again later.")
		return
	}
	writeEmailVerificationRequestAccepted(w)
}

func writeEmailVerificationRequestAccepted(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "If an unverified account exists for that email, a verification link will be sent.",
	})
}

func generateAuthToken() (string, [32]byte, error) {
	randomToken := make([]byte, 32)
	if _, err := rand.Read(randomToken); err != nil {
		return "", [32]byte{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(randomToken)
	return token, sha256.Sum256([]byte(token)), nil
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

func (s *Server) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var input passwordResetRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(input.Email) {
		writeError(w, http.StatusBadRequest, "invalid_email", "Enter a valid email address.")
		return
	}
	if !s.passwordResetEnabled() {
		writeError(w, http.StatusServiceUnavailable, "password_reset_unavailable", "Password reset email is not configured yet.")
		return
	}
	if s.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "The service is temporarily unavailable.")
		return
	}

	var userID string
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id::text FROM users
		WHERE lower(email) = $1 AND active = TRUE AND email_verified = TRUE
		  AND (password_hash IS NOT NULL OR google_subject IS NOT NULL)`,
		input.Email,
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResetRequestAccepted(w)
		return
	}
	if err != nil {
		log.Printf("find account for password reset: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password reset could not be requested. Please try again.")
		return
	}

	randomToken := make([]byte, 32)
	if _, err := rand.Read(randomToken); err != nil {
		log.Printf("generate password reset token: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password reset could not be requested. Please try again.")
		return
	}
	token := base64.RawURLEncoding.EncodeToString(randomToken)
	tokenHash := sha256.Sum256([]byte(token))
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin password reset request: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password reset could not be requested. Please try again.")
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM password_reset_tokens WHERE user_id = $1`, userID); err != nil {
		log.Printf("replace password reset token: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password reset could not be requested. Please try again.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash[:], userID, time.Now().Add(passwordResetDuration),
	); err != nil {
		log.Printf("save password reset token: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password reset could not be requested. Please try again.")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit password reset request: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password reset could not be requested. Please try again.")
		return
	}

	resetURL := strings.TrimRight(s.config.AppBaseURL, "/") + "/?password_reset_token=" + url.QueryEscape(token)
	if err := s.sendPasswordResetEmail(input.Email, resetURL); err != nil {
		log.Printf("send password reset email: %v", err)
		if _, deleteErr := s.db.ExecContext(r.Context(), `DELETE FROM password_reset_tokens WHERE token_hash = $1`, tokenHash[:]); deleteErr != nil {
			log.Printf("remove unsent password reset token: %v", deleteErr)
		}
		writeError(w, http.StatusServiceUnavailable, "password_reset_unavailable", "The reset email could not be sent. Please try again later.")
		return
	}
	writeResetRequestAccepted(w)
}

type confirmPasswordResetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s *Server) confirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var input confirmPasswordResetRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Token == "" || !validPassword(input.Password) {
		writeError(w, http.StatusBadRequest, "invalid_password_reset", "Use a valid reset link and a password of at least 8 characters.")
		return
	}
	if s.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "The service is temporarily unavailable.")
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("hash reset password: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password could not be updated.")
		return
	}
	tokenHash := sha256.Sum256([]byte(input.Token))
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin password reset confirmation: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password could not be updated.")
		return
	}
	defer tx.Rollback()
	var userID string
	err = tx.QueryRowContext(r.Context(), `
		DELETE FROM password_reset_tokens
		WHERE token_hash = $1 AND expires_at > now()
		RETURNING user_id::text`,
		tokenHash[:],
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "password_reset_expired", "This reset link is invalid or expired. Request a new one.")
		return
	}
	if err != nil {
		log.Printf("consume password reset token: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password could not be updated.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		UPDATE users SET password_hash = $1 WHERE id = $2 AND active = TRUE`,
		passwordHash, userID,
	)
	if err != nil {
		log.Printf("update password from reset link: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password could not be updated.")
		return
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			log.Printf("check password reset user update: %v", err)
			writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password could not be updated.")
		} else {
			writeError(w, http.StatusBadRequest, "password_reset_expired", "This reset link is invalid or expired. Request a new one.")
		}
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit password reset: %v", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "Password could not be updated.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Password updated. You can now sign in."})
}

func writeResetRequestAccepted(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "If an account exists for that email, a reset link will be sent.",
	})
}

func (s *Server) sendPasswordResetEmail(recipient, resetURL string) error {
	body := fmt.Sprintf(
		"Use this secure link to set or reset your CM RetailOS password. It expires in 30 minutes and can only be used once:\r\n\r\n%s\r\n\r\n"+
			"If you did not request this, you can ignore this email.\r\n",
		resetURL,
	)
	return s.sendSMTPEmail(recipient, "Set or reset your CM RetailOS password", body)
}

func (s *Server) sendEmailVerificationEmail(recipient, verificationURL string) error {
	body := fmt.Sprintf(
		"Verify your email address to activate your CM RetailOS account. This link expires in 24 hours and can only be used once:\r\n\r\n%s\r\n\r\n"+
			"If you did not create this account, you can ignore this email.\r\n",
		verificationURL,
	)
	return s.sendSMTPEmail(recipient, "Verify your CM RetailOS email", body)
}

func (s *Server) sendSMTPEmail(recipient, subject, body string) error {
	address := net.JoinHostPort(s.config.SMTPHost, s.config.SMTPPort)
	connection, err := (&net.Dialer{Timeout: 10 * time.Second}).Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return fmt.Errorf("set SMTP deadline: %w", err)
	}
	client, err := smtp.NewClient(connection, s.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("start SMTP session: %w", err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("SMTP server does not offer STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: s.config.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("secure SMTP session: %w", err)
	}
	auth := smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("authenticate with SMTP server: %w", err)
	}
	from, err := mail.ParseAddress(s.config.SMTPFrom)
	if err != nil {
		return fmt.Errorf("parse SMTP sender: %w", err)
	}
	to, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("parse email recipient: %w", err)
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("set email recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start email message: %w", err)
	}
	message := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from.String(), to.String(), subject, body,
	)
	if _, err := writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("write email message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("send email message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}
	return nil
}

func validEmail(value string) bool {
	if len(value) > 254 {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && !strings.ContainsAny(value, "\r\n")
}

func validPassword(value string) bool {
	return len([]rune(value)) >= minimumPasswordLength && len([]byte(value)) <= maximumPasswordBytes
}
