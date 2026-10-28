package identity

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"kitdesign/services/user/internal/apperr"
)

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 254 {
		return "", apperr.Invalid("Enter a valid email address.")
	}

	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", apperr.Invalid("Enter a valid email address.")
	}

	return email, nil
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < 12 {
		return apperr.Invalid("Password must be at least 12 characters.")
	}
	if utf8.RuneCountInString(password) > 128 || len(password) > 512 {
		return apperr.Invalid("Password must be at most 128 characters.")
	}
	return nil
}

func NormalizeDisplayName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", apperr.Invalid("Display name is required.")
	}
	if utf8.RuneCountInString(name) > 80 {
		return "", apperr.Invalid("Display name must be at most 80 characters.")
	}
	if strings.ContainsFunc(name, unicode.IsControl) {
		return "", apperr.Invalid("Display name contains invalid characters.")
	}
	return name, nil
}
