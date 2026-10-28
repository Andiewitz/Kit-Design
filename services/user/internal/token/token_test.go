package token_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"kitdesign/services/user/internal/token"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	raw, hash, err := token.Generate(32)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" {
		t.Fatal("raw token is empty")
	}
	if len(hash) != 32 {
		t.Fatalf("hash length = %d, want 32", len(hash))
	}

	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("token is not raw URL base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("decoded token length = %d, want 32", len(decoded))
	}

	if !bytes.Equal(hash, token.Hash(raw)) {
		t.Fatal("returned hash does not match Hash(raw)")
	}
}

func TestGenerateIsRandom(t *testing.T) {
	t.Parallel()

	first, _, err := token.Generate(32)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := token.Generate(32)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two generated tokens are identical")
	}
}

func TestHash(t *testing.T) {
	t.Parallel()

	if !bytes.Equal(token.Hash("value"), token.Hash("value")) {
		t.Fatal("hash is not deterministic")
	}
	if bytes.Equal(token.Hash("value"), token.Hash("other")) {
		t.Fatal("different values produced the same hash")
	}
}
