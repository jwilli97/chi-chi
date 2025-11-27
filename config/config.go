package config

import (
	"encoding/hex"
	"os"
	"strconv"
	"sync"
	"time"
)

// Config holds all configuration values
type Config struct {
	RedisURL              string
	RedisPassword         string
	ResyAPIKey            string
	CookieSecretKey       []byte
	CookieBlockKey        []byte
	Port                  string
	AdminToken            string
	CookieRefreshEnabled  bool
	CookieRefreshInterval time.Duration
	KnownVenueIDs         []int64
}

var (
	cfg  *Config
	once sync.Once
)

// Get returns the singleton configuration
func Get() *Config {
	once.Do(func() {
		cfg = &Config{
			RedisURL:              getEnv("REDIS_URL", "localhost:6379"),
			RedisPassword:         getEnv("REDIS_PASSWORD", ""),
			ResyAPIKey:            getEnv("RESY_API_KEY", "VbWk7s3L4KiK5fzlO7JD3Q5EYolJI7n5"),
			CookieSecretKey:       getSecretKey("COOKIE_SECRET_KEY"),
			CookieBlockKey:        getSecretKey("COOKIE_BLOCK_KEY"),
			Port:                  getEnv("PORT", "8090"),
			AdminToken:            getEnv("ADMIN_TOKEN", ""),
			CookieRefreshEnabled:  getEnvBool("COOKIE_REFRESH_ENABLED", true),
			CookieRefreshInterval: getEnvDuration("COOKIE_REFRESH_INTERVAL", 6*time.Hour),
			KnownVenueIDs:         []int64{89607, 89678, 92807},
		}
	})
	return cfg
}

// getEnv returns the environment variable value or a default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool returns a boolean from environment variable or default
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	// Accept "true", "1", "yes" as true; anything else as false
	return value == "true" || value == "1" || value == "yes"
}

// getEnvDuration returns a duration from environment variable or default
// Accepts formats like "6h", "30m", "1h30m"
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	// First try parsing as a Go duration string (e.g., "6h", "30m")
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}

	// Fall back to parsing as hours (e.g., "6" means 6 hours)
	if hours, err := strconv.Atoi(value); err == nil {
		return time.Duration(hours) * time.Hour
	}

	return defaultValue
}

// getSecretKey returns a 32-byte key from hex-encoded env var or nil if not set
func getSecretKey(key string) []byte {
	hexKey := os.Getenv(key)
	if hexKey == "" {
		return nil // Will trigger random key generation
	}
	decoded, err := hex.DecodeString(hexKey)
	if err != nil || len(decoded) != 32 {
		return nil
	}
	return decoded
}

// HasAdminToken returns true if an admin token is configured
func (c *Config) HasAdminToken() bool {
	return c.AdminToken != ""
}

// ValidateAdminToken checks if the provided token matches the configured admin token
func (c *Config) ValidateAdminToken(token string) bool {
	if !c.HasAdminToken() {
		return false // No admin token configured, deny all
	}
	return token == c.AdminToken
}
