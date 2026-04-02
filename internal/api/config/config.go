package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the API server
type Config struct {
	Server   ServerConfig
	Security SecurityConfig
	CORS     CORSConfig
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	APIKey          string
	RateLimit       int
	RateLimitWindow time.Duration
	EnableHTTPS     bool
	TLSCertFile     string
	TLSKeyFile      string
	JWTSecret       string
	TokenExpiration time.Duration
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// Load loads configuration from environment variables with defaults
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("BACKEND_PORT", "8080"),
			ReadTimeout:  getDurationEnv("READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDurationEnv("WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDurationEnv("IDLE_TIMEOUT", 60*time.Second),
		},
		Security: SecurityConfig{
			APIKey:          getEnv("API_KEY", generateDefaultAPIKey()),
			RateLimit:       getIntEnv("RATE_LIMIT", 100),
			RateLimitWindow: getDurationEnv("RATE_LIMIT_WINDOW", 1*time.Minute),
			EnableHTTPS:     getBoolEnv("ENABLE_HTTPS", false),
			TLSCertFile:     getEnv("TLS_CERT_FILE", ""),
			TLSKeyFile:      getEnv("TLS_KEY_FILE", ""),
			JWTSecret:       getEnv("JWT_SECRET", generateDefaultJWTSecret()),
			TokenExpiration: getDurationEnv("TOKEN_EXPIRATION", 24*time.Hour),
		},
		CORS: CORSConfig{
			AllowedOrigins:   getSliceEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:8080"}),
			AllowedMethods:   getSliceEnv("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			AllowedHeaders:   getSliceEnv("CORS_ALLOWED_HEADERS", []string{"Content-Type", "Authorization", "X-API-Key"}),
			AllowCredentials: getBoolEnv("CORS_ALLOW_CREDENTIALS", true),
			MaxAge:           getIntEnv("CORS_MAX_AGE", 86400),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple comma-separated parsing
		// In production, use a proper CSV parser
		result := []string{}
		current := ""
		for _, char := range value {
			if char == ',' {
				if current != "" {
					result = append(result, current)
					current = ""
				}
			} else {
				current += string(char)
			}
		}
		if current != "" {
			result = append(result, current)
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

func generateDefaultAPIKey() string {
	// In production, use a cryptographically secure random generator
	// This is a fallback for development only
	return "dev-api-key-change-in-production"
}

func generateDefaultJWTSecret() string {
	// In production, use a cryptographically secure random generator
	// This is a fallback for development only
	return "dev-jwt-secret-change-in-production"
}
