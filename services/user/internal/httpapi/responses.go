package httpapi

import (
	"time"

	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/session"
)

type userResponse struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	DisplayName     string     `json:"display_name"`
	Status          string     `json:"status"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

func newUserResponse(user identity.User) userResponse {
	return userResponse{
		ID:              user.ID.String(),
		Email:           user.Email,
		DisplayName:     user.DisplayName,
		Status:          string(user.Status),
		EmailVerifiedAt: user.EmailVerifiedAt,
		LastLoginAt:     user.LastLoginAt,
		CreatedAt:       user.CreatedAt,
	}
}

type tokenResponse struct {
	AccessToken string        `json:"access_token"`
	TokenType   string        `json:"token_type"`
	ExpiresIn   int64         `json:"expires_in"`
	User        *userResponse `json:"user,omitempty"`
}

func newTokenResponse(tokens session.Tokens, user *identity.User) tokenResponse {
	expiresIn := int64(time.Until(tokens.AccessExpiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}
	response := tokenResponse{
		AccessToken: tokens.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	}
	if user != nil {
		value := newUserResponse(*user)
		response.User = &value
	}
	return response
}

type sessionResponse struct {
	ID         string    `json:"id"`
	Current    bool      `json:"current"`
	UserAgent  string    `json:"user_agent"`
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type auditResponse struct {
	ID        string         `json:"id"`
	Action    string         `json:"action"`
	IP        string         `json:"ip,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func newAuditResponse(event audit.Event) auditResponse {
	return auditResponse{
		ID:        event.ID.String(),
		Action:    event.Action,
		IP:        event.IP,
		Metadata:  event.Metadata,
		CreatedAt: event.CreatedAt,
	}
}
