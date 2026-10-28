package session

import (
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
)

type Claims struct {
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	SessionID   string   `json:"sid"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	privateKey ed25519.PrivateKey
	keyID      string
	issuer     string
	audience   string
	accessTTL  time.Duration
}

func NewTokenManager(privateKey ed25519.PrivateKey, keyID, issuer, audience string, accessTTL time.Duration) (*TokenManager, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid ed25519 private key")
	}
	if accessTTL <= 0 {
		return nil, errors.New("access token TTL must be positive")
	}
	return &TokenManager{
		privateKey: privateKey,
		keyID:      keyID,
		issuer:     issuer,
		audience:   audience,
		accessTTL:  accessTTL,
	}, nil
}

func (m *TokenManager) Issue(userID, sessionID uuid.UUID, roles, permissions []string) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTTL)
	claims := Claims{
		Roles:       roles,
		Permissions: permissions,
		SessionID:   sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.Must(uuid.NewV7()).String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = m.keyID
	signed, err := token.SignedString(m.privateKey)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (m *TokenManager) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(
		raw,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
				return nil, errors.New("unexpected signing method")
			}
			return m.privateKey.Public(), nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(5*time.Second),
	)
	if err != nil || !parsed.Valid {
		return nil, apperr.Unauthorized("invalid_token")
	}
	if claims.Subject == "" || claims.SessionID == "" {
		return nil, apperr.Unauthorized("invalid_token")
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, apperr.Unauthorized("invalid_token")
	}
	if _, err := uuid.Parse(claims.SessionID); err != nil {
		return nil, apperr.Unauthorized("invalid_token")
	}
	return claims, nil
}
