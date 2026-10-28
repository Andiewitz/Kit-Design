package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func Generate(size int) (string, []byte, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	return value, Hash(value), nil
}

func Hash(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}
