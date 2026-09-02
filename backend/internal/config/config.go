package config

import (
	"os"
	"strings"
)

// Config is read once at boot. Everything comes from the environment so the
// same image runs unchanged in every context.
//
// Note what is not here any more: a model API key. Coaching runs on each
// athlete's own provider account, so the only model secret the server holds is
// the one it seals those accounts with.
type Config struct {
	DatabaseURL string
	Addr        string
	// CredentialsKey seals the athletes' provider keys at rest. 32 bytes,
	// base64 or hex. Without it the coaching features are off for everyone.
	CredentialsKey string
	// AIThinking is "adaptive" (every current model) or "off" for a model old
	// enough to reject the parameter.
	AIThinking        string
	SearchToolVersion string
	SecureCookies     bool
	// AppOrigin is the public https origin of this site. Sign-in redirects are
	// built from it, so it has to match what is registered with the provider.
	AppOrigin string
	// Signing in with Google or ChatGPT is identity only — it buys no model
	// access, which still comes from each athlete's own key. Empty client
	// credentials mean that button is not offered at all.
	//
	// The issuer overrides exist because an issuer URL is exactly the kind of
	// detail a provider documents in one place and changes in another: set it
	// and the endpoints are rediscovered from there, with no code change.
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleIssuer        string
	ChatGPTClientID     string
	ChatGPTClientSecret string
	ChatGPTIssuer       string
}

func Load() Config {
	return Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		Addr:                envOr("API_ADDR", ":8080"),
		CredentialsKey:      os.Getenv("AI_CREDENTIALS_KEY"),
		AIThinking:          envOr("AI_THINKING", envOr("ANTHROPIC_THINKING", "adaptive")),
		SearchToolVersion:   envOr("WEB_SEARCH_TOOL_VERSION", "web_search_20250305"),
		AppOrigin:           os.Getenv("APP_ORIGIN"),
		GoogleClientID:      os.Getenv("OAUTH_GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("OAUTH_GOOGLE_CLIENT_SECRET"),
		GoogleIssuer:        os.Getenv("OAUTH_GOOGLE_ISSUER"),
		ChatGPTClientID:     os.Getenv("OAUTH_CHATGPT_CLIENT_ID"),
		ChatGPTClientSecret: os.Getenv("OAUTH_CHATGPT_CLIENT_SECRET"),
		ChatGPTIssuer:       os.Getenv("OAUTH_CHATGPT_ISSUER"),
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
