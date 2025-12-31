// Package config provides constants, environment variable handling, and XDG paths.
//
// OAuth Configuration:
// The OAuth client ID and authentication flow are derived from OpenAI's
// open-source Codex CLI (https://github.com/openai/codex), which is
// released under the Apache 2.0 License. This project uses the same
// authentication mechanism for interoperability purposes.
package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// OAuth constants
// DefaultOAuthClientID is from OpenAI's open-source Codex CLI
// (https://github.com/openai/codex - Apache 2.0 License).
// This is the same client ID used by OpenAI's official CLI tool.
// Users may override with OPENCOMPAT_OAUTH_CLIENT_ID environment variable.
const (
	DefaultOAuthClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	OAuthIssuer          = "https://auth.openai.com"
	OAuthTokenURL        = "https://auth.openai.com/oauth/token"
	OAuthAuthorizeURL    = "https://auth.openai.com/oauth/authorize"
	OAuthCallbackPort    = 1455
	OAuthRedirectURI     = "http://localhost:1455/auth/callback"
	OAuthScopes          = "openid profile email offline_access"
)

// ChatGPT Backend constants
const (
	ChatGPTBaseURL      = "https://chatgpt.com/backend-api"
	ChatGPTResponsesURL = "https://chatgpt.com/backend-api/codex/responses"
)

// GitHub constants for instructions (Apache 2.0 licensed content)
const (
	GitHubReleasesAPI  = "https://api.github.com/repos/openai/codex/releases/latest"
	GitHubReleasesHTML = "https://github.com/openai/codex/releases/latest"
	GitHubRawBaseURL   = "https://raw.githubusercontent.com/openai/codex"
)

// JWT claim path
const JWTClaimPath = "https://api.openai.com/auth"

// Application name for XDG paths
const AppName = "opencompat"

// Default server configuration
const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 8080
)

// Cache TTL in minutes
const (
	InstructionsDiskCacheTTL   = 7 * 24 * 60 // 7 days for disk cache
	DefaultInstructionsRefresh = 24 * 60     // 24 hours for background refresh
)

// Config holds runtime configuration from environment variables.
type Config struct {
	Host                string
	Port                int
	Verbose             bool
	ReasoningEffort     string
	ReasoningSummary    string
	ReasoningCompat     string // "none", "think-tags", "o3", "legacy"
	TextVerbosity       string
	InstructionsRefresh int    // in minutes
	OAuthClientID       string // configurable OAuth client ID
}

// Load reads configuration from environment variables with defaults.
func Load() *Config {
	cfg := &Config{
		Host:                getEnv("OPENCOMPAT_HOST", DefaultHost),
		Port:                getEnvInt("OPENCOMPAT_PORT", DefaultPort),
		Verbose:             getEnvBool("OPENCOMPAT_VERBOSE", false),
		ReasoningEffort:     getEnv("OPENCOMPAT_REASONING_EFFORT", "medium"),
		ReasoningSummary:    getEnv("OPENCOMPAT_REASONING_SUMMARY", "auto"),
		ReasoningCompat:     getEnv("OPENCOMPAT_REASONING_COMPAT", "none"),
		TextVerbosity:       getEnv("OPENCOMPAT_TEXT_VERBOSITY", "medium"),
		InstructionsRefresh: getEnvInt("OPENCOMPAT_INSTRUCTIONS_REFRESH", DefaultInstructionsRefresh),
		OAuthClientID:       getEnv("OPENCOMPAT_OAUTH_CLIENT_ID", DefaultOAuthClientID),
	}
	return cfg
}

// DataDir returns the XDG data directory for the application.
// Uses $XDG_DATA_HOME/opencompat or ~/.local/share/opencompat
func DataDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, AppName)
}

// CacheDir returns the XDG cache directory for the application.
// Uses $XDG_CACHE_HOME/opencompat or ~/.cache/opencompat
func CacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, AppName)
}

// AuthFilePath returns the path to the auth.json file.
func AuthFilePath() string {
	return filepath.Join(DataDir(), "auth.json")
}

// EnsureDataDir creates the data directory if it doesn't exist.
func EnsureDataDir() error {
	return os.MkdirAll(DataDir(), 0700)
}

// EnsureCacheDir creates the cache directory if it doesn't exist.
func EnsureCacheDir() error {
	return os.MkdirAll(CacheDir(), 0700)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}
