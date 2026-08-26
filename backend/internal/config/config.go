package config

import (
	"os"
	"strings"
)

// Config is read once at boot. Everything comes from the environment so the
// same image runs unchanged in every context.
type Config struct {
	DatabaseURL    string
	Addr           string
	AnthropicKey   string
	AnthropicModel string
	SearchToolVersion string
	SecureCookies  bool
	AppOrigin      string
}

func Load() Config {
	return Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		Addr:           envOr("API_ADDR", ":8080"),
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel: envOr("ANTHROPIC_MODEL", "claude-sonnet-5"),
		SearchToolVersion: envOr("WEB_SEARCH_TOOL_VERSION", "web_search_20250305"),
		AppOrigin:      os.Getenv("APP_ORIGIN"),
		// Cookies are Secure unless explicitly disabled for a plain-HTTP test box.
		SecureCookies: !strings.EqualFold(os.Getenv("INSECURE_COOKIES"), "true"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
