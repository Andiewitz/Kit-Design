package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env            string
	Port           string
	DatabaseURL    string
	AppOrigin      string
	PublicAppURL   string
	JWTPrivateKey  ed25519.PrivateKey
	JWTKeyID       string
	JWTIssuer      string
	JWTAudience    string
	JWTKeyIsRandom bool
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	CookieSecure   bool
	CookieDomain   string
	Mailer         string
	SMTPAddr       string
	MailFrom       string
	InternalToken  string
	RateLimit      bool
	LogLevel       slog.Level
	AutoMigrate    bool
}

func Load() (Config, error) {
	env := getenv("APP_ENV", "development")
	cfg := Config{
		Env:           env,
		Port:          getenv("PORT", "3001"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		AppOrigin:     strings.TrimRight(getenv("APP_ORIGIN", "http://localhost:5173"), "/"),
		PublicAppURL:  strings.TrimRight(getenv("PUBLIC_APP_URL", "http://localhost:5173"), "/"),
		JWTKeyID:      getenv("JWT_KEY_ID", "dev-1"),
		JWTIssuer:     getenv("JWT_ISSUER", "kit-user"),
		JWTAudience:   getenv("JWT_AUDIENCE", "kit-api"),
		CookieDomain:  os.Getenv("COOKIE_DOMAIN"),
		Mailer:        getenv("MAILER", "log"),
		SMTPAddr:      getenv("SMTP_ADDR", "localhost:1025"),
		MailFrom:      getenv("MAIL_FROM", "no-reply@kit.local"),
		InternalToken: os.Getenv("INTERNAL_SERVICE_TOKEN"),
	}

	if cfg.DatabaseURL == "" {
		if env == "production" {
			return Config{}, errors.New("DATABASE_URL is required")
		}
		cfg.DatabaseURL = "postgres://kit:kit_dev_password@localhost:5432/kit?sslmode=disable"
	}

	accessTTL, err := durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	cfg.AccessTTL = accessTTL

	refreshTTL, err := durationEnv("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.RefreshTTL = refreshTTL

	if cfg.RefreshTTL <= cfg.AccessTTL {
		return Config{}, errors.New("REFRESH_TOKEN_TTL must be greater than ACCESS_TOKEN_TTL")
	}

	secureDefault := env == "production"
	cfg.CookieSecure, err = boolEnv("COOKIE_SECURE", secureDefault)
	if err != nil {
		return Config{}, err
	}
	cfg.RateLimit, err = boolEnv("RATE_LIMIT_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	cfg.AutoMigrate, err = boolEnv("AUTO_MIGRATE", false)
	if err != nil {
		return Config{}, err
	}

	level, err := logLevel(getenv("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level

	if cfg.Mailer != "log" && cfg.Mailer != "smtp" {
		return Config{}, fmt.Errorf("MAILER must be log or smtp, got %q", cfg.Mailer)
	}

	cfg.JWTPrivateKey, cfg.JWTKeyIsRandom, err = loadPrivateKey(cfg.Env, os.Getenv("JWT_PRIVATE_KEY"))
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func boolEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func logLevel(value string) (slog.Level, error) {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown LOG_LEVEL %q", value)
	}
}

func loadPrivateKey(env, value string) (ed25519.PrivateKey, bool, error) {
	if strings.TrimSpace(value) == "" {
		if env == "production" {
			return nil, false, errors.New("JWT_PRIVATE_KEY is required in production")
		}
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, false, err
		}
		return privateKey, true, nil
	}

	decoded, err := decodeBase64(strings.TrimSpace(value))
	if err != nil {
		return nil, false, fmt.Errorf("JWT_PRIVATE_KEY: %w", err)
	}

	switch len(decoded) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), false, nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), false, nil
	default:
		return nil, false, fmt.Errorf("JWT_PRIVATE_KEY must decode to %d-byte seed or %d-byte key", ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}

func decodeBase64(value string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
