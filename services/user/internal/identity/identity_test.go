package identity_test

import (
	"errors"
	"strings"
	"testing"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/identity"
)

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "normalizes case and spaces", input: "  User@Example.COM ", want: "user@example.com"},
		{name: "empty", input: "", wantErr: true},
		{name: "missing at", input: "user.example.com", wantErr: true},
		{name: "display name form", input: "User <user@example.com>", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 250) + "@example.com", wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := identity.NormalizeEmail(test.input)
			if test.wantErr {
				if err == nil || !apperr.IsKind(err, apperr.KindValidation) {
					t.Fatalf("expected validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	if err := identity.ValidatePassword(strings.Repeat("a", 11)); err == nil {
		t.Fatal("expected short password to fail")
	}
	if err := identity.ValidatePassword(strings.Repeat("a", 12)); err != nil {
		t.Fatalf("expected 12-character password to pass: %v", err)
	}
	if err := identity.ValidatePassword(strings.Repeat("a", 129)); err == nil {
		t.Fatal("expected long password to fail")
	}
	if err := identity.ValidatePassword(strings.Repeat("é", 12)); err != nil {
		t.Fatalf("expected 12 runes to pass: %v", err)
	}
}

func TestNormalizeDisplayName(t *testing.T) {
	t.Parallel()

	got, err := identity.NormalizeDisplayName("  Ada Lovelace  ")
	if err != nil || got != "Ada Lovelace" {
		t.Fatalf("got %q, err %v", got, err)
	}
	if _, err := identity.NormalizeDisplayName("   "); err == nil {
		t.Fatal("expected empty name to fail")
	}
	if _, err := identity.NormalizeDisplayName("Ada\nLovelace"); err == nil {
		t.Fatal("expected control character to fail")
	}
	if _, err := identity.NormalizeDisplayName(strings.Repeat("a", 81)); err == nil {
		t.Fatal("expected long name to fail")
	}
}

func TestPasswordHashRoundTrip(t *testing.T) {
	t.Parallel()

	hash, err := identity.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}

	valid, err := identity.VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !valid {
		t.Fatalf("expected password to verify, valid=%v err=%v", valid, err)
	}

	valid, err = identity.VerifyPassword(hash, "wrong password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Fatal("expected wrong password to fail")
	}

	if _, err := identity.VerifyPassword("not-a-hash", "password"); !errors.Is(err, identity.ErrInvalidHash) {
		t.Fatalf("expected ErrInvalidHash, got %v", err)
	}
}

func TestStatusValid(t *testing.T) {
	t.Parallel()

	for _, status := range []identity.Status{
		identity.StatusPending,
		identity.StatusActive,
		identity.StatusSuspended,
		identity.StatusDeleted,
	} {
		if !status.Valid() {
			t.Fatalf("expected %q to be valid", status)
		}
	}
	if identity.Status("unknown").Valid() {
		t.Fatal("expected unknown status to be invalid")
	}
}
