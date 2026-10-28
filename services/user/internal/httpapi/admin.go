package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/session"
)

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := parseQueryInt(r, "limit", 50, 1, 100)
	offset := parseQueryInt(r, "offset", 0, 0, 1_000_000)

	users, total, err := s.authorization.ListUsers(r.Context(), query, limit, offset)
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	response := make([]userResponse, 0, len(users))
	for _, user := range users {
		response = append(response, newUserResponse(user))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users":  response,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleAdminSetStatus(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	actorID, targetID, ok := s.adminIDs(w, r)
	if !ok {
		return
	}

	var request struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	user, err := s.authorization.SetStatus(r.Context(), actorID, targetID, identity.Status(request.Status), clientIP(r))
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	writeJSON(w, http.StatusOK, newUserResponse(user))
}

func (s *Server) handleAdminGrantRole(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	actorID, targetID, ok := s.adminIDs(w, r)
	if !ok {
		return
	}

	var request struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, s.logger)
		return
	}

	if err := s.authorization.GrantRole(r.Context(), actorID, targetID, request.Role, clientIP(r)); err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminRevokeRole(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	actorID, targetID, ok := s.adminIDs(w, r)
	if !ok {
		return
	}

	if err := s.authorization.RevokeRole(r.Context(), actorID, targetID, r.PathValue("role"), clientIP(r)); err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.authorization.ListRoles(r.Context())
	if err != nil {
		writeError(w, r, err, s.logger)
		return
	}
	if roles == nil {
		roles = []identity.Role{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": roles})
}

func (s *Server) adminIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	claims, ok := session.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, r, apperr.Unauthorized("invalid_token"), s.logger)
		return uuid.Nil, uuid.Nil, false
	}
	actorID, err := uuid.Parse(claims.Subject)
	if err != nil {
		writeError(w, r, apperr.Unauthorized("invalid_token"), s.logger)
		return uuid.Nil, uuid.Nil, false
	}
	targetID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, apperr.NotFound("user_not_found"), s.logger)
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, targetID, true
}

func parseQueryInt(r *http.Request, key string, fallback, minimum, maximum int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}
