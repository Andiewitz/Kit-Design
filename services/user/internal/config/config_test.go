package config_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"kitdesign/services/user/internal/config"
)

var envKeys = []string{
	"APP_ENV", "PORT", "DATABASE_URL", "APP_ORIGIN", "PUBLIC_APP_URL",
	"JWT_PRIVATE_KEY", "JWT_KEY_ID", "JWT_ISSUER", "JWT_AUDIENCE",
	"ACCESS_TOKEN_TTL", "REFRESH_TOKEN_TTL", "COOKIE_SECURE", "COOKIE_DOMAIN",
	"MAILER", "SMTP_ADDR", "MAIL_FROM", "INTERNAL_SERVICE_TOKEN",
	"RATE_LIMIT_ENABLED", "AUTO_MIGRATE", "LOG_LEVEL",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range envKeys {
		t.Setenv(key, "")
	}
}

func TestLoadDevelopmentDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "development" {
		t.Fatalf("env = %q", cfg.Env)
	}
	if cfg.Port != "3001" {
		t.Fatalf("port = %q", cfg.Port)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("development database URL is empty")
	}
	if !cfg.JWTKeyIsRandom {
		t.Fatal("expected ephemeral JWT key")
	}
	if len(cfg.JWTPrivateKey) != ed25519.PrivateKeySize {
		t.Fatalf("private key length = %d", len(cfg.JWTPrivateKey))
	}
	if cfg.CookieSecure {
		t.Fatal("development cookies should not require HTTPS by default")
	}
}

func TestLoadProductionRequiresSecrets(t *testing.T) {
	clearEnv(t)
	t.Setenv("APP_ENV", "production")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected production without database and JWT key to fail")
	}

	t.Setenv("DATABASE_URL", "postgres://example")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected production without JWT key to fail")
	}
}

func TestLoadPrivateKey(t *testing.T) {
	clearEnv(t)
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_PRIVATE_KEY", base64.StdEncoding.EncodeToString(privateKey.Seed()))

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWTKeyIsRandom {
		t.Fatal("expected configured key")
	}
	if !cfg.JWTPrivateKey.Public().(ed25519.PublicKey).Equal(privateKey.Public()) {
		t.Fatal("loaded public key does not match")
	}
	if !cfg.CookieSecure {
		t.Fatal("production cookies should require HTTPS by default")
	}
}

func TestLoadRejectsInvalidSettings(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "invalid access ttl", key: "ACCESS_TOKEN_TTL", value: "soon"},
		{name: "refresh shorter than access", key: "REFRESH_TOKEN_TTL", value: "1m"},
		{name: "invalid mailer", key: "MAILER", value: "carrier-pigeon"},
		{name: "invalid log level", key: "LOG_LEVEL", value: "verbose"},
		{name: "invalid jwt key", key: "JWT_PRIVATE_KEY", value: "not-base64!"},
		{name: "invalid secure cookie flag", key: "COOKIE_SECURE", value: "yes-please"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(test.key, test.value)
			if _, err := config.Load(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
