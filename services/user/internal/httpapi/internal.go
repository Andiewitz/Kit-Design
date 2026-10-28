package httpapi

import (
	"crypto/subtle"
	"net/http"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
)

func (s *Server) handleInternalAuthz(w http.ResponseWriter, r *http.Request) {
	if s.config.InternalToken == "" {
		writeError(w, r, apperr.NotFound("not_found"), s.logger)
		return
	}
	provided := r.Header.Get("X-Internal-Token")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(s.config.InternalToken)) != 1 {
		writeError(w, r, apperr.Unauthorized("invalid_internal_token"), s.logger)
		return
	}

	var request struct {
		UserID     string `json:"user_id"`
		Permission string `json:"permission"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	userID, err := uuid.Parse(request.UserID)
	if err != nil {
		writeError(w, r, apperr.Invalid("user_id must be a UUID."), s.logger)
		return
	}

	allowed, err := s.authorization.CheckPermission(r.Context(), userID, request.Permission)
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"allowed": allowed})
}
