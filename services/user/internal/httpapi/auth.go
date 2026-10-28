package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/session"
)

const refreshCookieName = "kit_refresh"

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	_, err := s.register.Register(r.Context(), request.Email, request.Password, clientIP(r))
	if err != nil && apperr.Code(err) != "email_exists" {
		writeError(w, r, err, s.logger)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "pending_verification",
		"message": "If the address can be registered, a verification email has been sent.",
	})
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	user, err := s.register.VerifyEmail(r.Context(), request.Token, clientIP(r))
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "active", "user": newUserResponse(user)})
}

func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	if err := s.register.ResendVerification(r.Context(), request.Email, clientIP(r)); err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "pending_verification",
		"message": "If the account is awaiting verification, a new email has been sent.",
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	tokens, user, err := s.login.Login(r.Context(), request.Email, request.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	s.setRefreshCookie(w, tokens.RefreshToken, tokens.RefreshExpiresAt)
	writeJSON(w, http.StatusOK, newTokenResponse(tokens, &user))
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, r, apperr.Unauthorized("invalid_token"), s.logger)
		return
	}

	tokens, err := s.sessions.Refresh(r.Context(), cookie.Value, clientIP(r))
	if err != nil {
		s.clearRefreshCookie(w)
		writeError(w, r, err, s.logger)
		return
	}

	s.setRefreshCookie(w, tokens.RefreshToken, tokens.RefreshExpiresAt)
	writeJSON(w, http.StatusOK, newTokenResponse(tokens, nil))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	claims, ok := session.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, r, apperr.Unauthorized("invalid_token"), s.logger)
		return
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		writeError(w, r, apperr.Unauthorized("invalid_token"), s.logger)
		return
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		writeError(w, r, apperr.Unauthorized("invalid_token"), s.logger)
		return
	}

	if err := s.sessions.Revoke(r.Context(), sessionID, userID, "logout"); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	if err := s.recovery.ForgotPassword(r.Context(), request.Email, clientIP(r)); err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"message": "If an account exists for that address, a reset email has been sent.",
	})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	var request struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	if err := s.recovery.ResetPassword(r.Context(), request.Token, request.Password, clientIP(r)); err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	s.clearRefreshCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
}

func (s *Server) setRefreshCookie(w http.ResponseWriter, value string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     "/v1/auth",
		Domain:   s.config.CookieDomain,
		Expires:  expires.UTC(),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/v1/auth",
		Domain:   s.config.CookieDomain,
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
