package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"psv-crowd-counter/internal/api/config"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// APIKeyContextKey is the context key for authenticated API key
	APIKeyContextKey contextKey = "api_key"
)

// Middleware is a function that wraps an http.Handler
type Middleware func(http.Handler) http.Handler

// Chain chains multiple middleware together
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// CORS returns a middleware that handles Cross-Origin Resource Sharing
func CORS(cfg config.CORSConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range cfg.AllowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
			w.Header().Set("Access-Control-Allow-Credentials", fmt.Sprintf("%t", cfg.AllowCredentials))
			w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders returns a middleware that adds security headers
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Enable XSS filter
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// Strict Transport Security (only for HTTPS)
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			// Content Security Policy
			w.Header().Set("Content-Security-Policy", "default-src 'self'")

			// Referrer Policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions Policy
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   int
	window  time.Duration
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		limit:   limit,
		window:  window,
	}
}

// RateLimit returns a middleware that rate limits requests
func (rl *RateLimiter) RateLimit() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client identifier (IP address or API key)
			clientID := getClientID(r)

			rl.mu.Lock()
			b, exists := rl.buckets[clientID]
			if !exists {
				b = &bucket{
					tokens:    rl.limit,
					lastReset: time.Now(),
				}
				rl.buckets[clientID] = b
			}

			// Reset bucket if window has passed
			if time.Since(b.lastReset) > rl.window {
				b.tokens = rl.limit
				b.lastReset = time.Now()
			}

			// Check if request is allowed
			if b.tokens <= 0 {
				rl.mu.Unlock()
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.window.Seconds())))
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Consume a token
			b.tokens--
			rl.mu.Unlock()

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", b.tokens))

			next.ServeHTTP(w, r)
		})
	}
}

// getClientID extracts client identifier from request
func getClientID(r *http.Request) string {
	// Try to get API key first
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		return "api:" + apiKey
	}

	// Fall back to IP address
	ip := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		// Take the first IP in the list
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			ip = strings.TrimSpace(ips[0])
		}
	}
	return "ip:" + ip
}

// Authenticate returns a middleware that validates API key authentication
func Authenticate(apiKey string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for health endpoint
			if r.URL.Path == "/health" || r.URL.Path == "/api/v1/health" {
				next.ServeHTTP(w, r)
				return
			}

			// Get API key from header
			providedKey := r.Header.Get("X-API-Key")
			if providedKey == "" {
				// Try Authorization header as fallback
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					providedKey = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			// Validate API key
			if providedKey == "" {
				log.Printf("Authentication failed: No API key provided for %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "Unauthorized: API key required", http.StatusUnauthorized)
				return
			}

			if providedKey != apiKey {
				log.Printf("Authentication failed: Invalid API key for %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
				http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
				return
			}

			// Add API key to context
			ctx := context.WithValue(r.Context(), APIKeyContextKey, providedKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestLogger returns a middleware that logs HTTP requests
func RequestLogger() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer wrapper to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			log.Printf(
				"[%s] %s %s %d %s %s",
				r.Method,
				r.URL.Path,
				r.RemoteAddr,
				wrapped.statusCode,
				duration.String(),
				r.UserAgent(),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Recovery returns a middleware that recovers from panics
func Recovery() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("Panic recovered: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
