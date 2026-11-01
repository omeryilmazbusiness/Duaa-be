package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env             string
	Version         string
	HTTPAddr        string
	ShutdownTimeout time.Duration
	JWTSecret       string
	JWTTTL          time.Duration
	// EncryptionKey seals secrets at rest (AI provider keys). Rotating it
	// invalidates stored keys, which must then be re-entered.
	EncryptionKey string
	// Optional overrides for OpenAI-compatible gateways or egress proxies.
	OpenAIBaseURL string
	GeminiBaseURL string
	DatabaseURL   string
	// DataDir holds local files when Postgres is not configured (sealed AI keys).
	DataDir string
	// PublicAppURL is the guest-facing origin encoded into printed QR codes.
	PublicAppURL string
	LogLevel     string
	LogFormat    string
	CORSOrigins  []string
}

func Load() (Config, error) {
	_ = godotenv.Load()
	cfg := Config{
		Env:             getenv("APP_ENV", "development"),
		Version:         getenv("APP_VERSION", "0.1.0"),
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		ShutdownTimeout: duration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		JWTSecret:       getenv("JWT_SECRET", ""),
		JWTTTL:          duration("JWT_TTL", 8*time.Hour),
		EncryptionKey:   getenv("APP_ENCRYPTION_KEY", ""),
		OpenAIBaseURL:   getenv("AI_OPENAI_BASE_URL", ""),
		GeminiBaseURL:   getenv("AI_GEMINI_BASE_URL", ""),
		DatabaseURL:     getenv("DATABASE_URL", ""),
		DataDir:         getenv("DUA_DATA_DIR", "data"),
		PublicAppURL:    getenv("PUBLIC_APP_URL", "http://localhost:3000"),
		LogLevel:        getenv("LOG_LEVEL", "info"),
		LogFormat:       getenv("LOG_FORMAT", "text"),
		CORSOrigins:     split(getenv("CORS_ORIGINS", "http://localhost:3000")),
	}
	if len(cfg.JWTSecret) < 32 {
		if cfg.Env == "production" {
			return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
		}
		if cfg.JWTSecret == "" {
			cfg.JWTSecret = "dev-only-change-me-to-at-least-32-chars"
		}
	}
	if len(cfg.EncryptionKey) < 32 {
		if cfg.Env == "production" {
			return Config{}, fmt.Errorf("APP_ENCRYPTION_KEY must be at least 32 characters in production")
		}
		if cfg.EncryptionKey == "" {
			cfg.EncryptionKey = "dev-only-encryption-key-change-me-32+chars"
		}
	}
	return cfg, nil
}

func (c Config) Production() bool { return c.Env == "production" }

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func split(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
