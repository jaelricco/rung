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
	// AnthropicThinking is "adaptive" (every current model) or "off" for a
	// model old enough to reject the adaptive thinking parameter.
	AnthropicThinking string
	SearchToolVersion string
	SecureCookies     bool
	AppOrigin         string
}

func Load() Config {
	return Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		Addr:              envOr("API_ADDR", ":8080"),
		AnthropicKey:      os.Getenv("ANTHROPIC_API_KEY"),
		// Opus by default: plan writing is the app's hardest call — a ladder
		// of progressions weighed against one athlete's records — and it is
		// where the difference between model tiers actually shows up.
		AnthropicModel:    envOr("ANTHROPIC_MODEL", "claude-opus-5"),
		AnthropicThinking: envOr("ANTHROPIC_THINKING", "adaptive"),
		SearchToolVersion: envOr("WEB_SEARCH_TOOL_VERSION", "web_search_20250305"),
		AppOrigin:         os.Getenv("APP_ORIGIN"),
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
