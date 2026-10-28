package httpapi

import (
	"net/http"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/session"
)

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
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

	user, err := s.account.Profile(r.Context(), userID)
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	writeJSON(w, http.StatusOK, newUserResponse(user))
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
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

	var request struct {
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	user, err := s.account.UpdateProfile(r.Context(), userID, request.DisplayName, clientIP(r))
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	writeJSON(w, http.StatusOK, newUserResponse(user))
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
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

	sessions, err := s.account.Sessions(r.Context(), userID)
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	response := make([]sessionResponse, 0, len(sessions))
	for _, item := range sessions {
		response = append(response, sessionResponse{
			ID:         item.ID.String(),
			Current:    item.ID.String() == claims.SessionID,
			UserAgent:  item.UserAgent,
			IP:         item.IP,
			CreatedAt:  item.CreatedAt,
			LastUsedAt: item.LastUsedAt,
			ExpiresAt:  item.ExpiresAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": response})
}

func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
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
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, apperr.NotFound("session_not_found"), s.logger)
		return
	}

	if err := s.account.RevokeSession(r.Context(), userID, sessionID, clientIP(r)); err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	if sessionID.String() == claims.SessionID {
		s.clearRefreshCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAccountEvents(w http.ResponseWriter, r *http.Request) {
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

	events, err := s.account.Events(r.Context(), userID, 50)
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	response := make([]auditResponse, 0, len(events))
	for _, event := range events {
		response = append(response, newAuditResponse(event))
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": response})
}
