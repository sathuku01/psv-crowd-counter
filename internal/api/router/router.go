package router

import (
	"net/http"

	"psv-crowd-counter/internal/api/config"
	"psv-crowd-counter/internal/api/handlers"
	"psv-crowd-counter/internal/api/middleware"

	"psv-crowd-counter/internal/service"
	"psv-crowd-counter/internal/storage"
)

// Router sets up all API routes with middleware
type Router struct {
	config  *config.Config
	handler *handlers.Handler
	limiter *middleware.RateLimiter
}

// NewRouter creates a new Router instance
func NewRouter(cfg *config.Config, store storage.Store, processor *service.Processor) *Router {
	handler := handlers.NewHandler(store, processor)
	limiter := middleware.NewRateLimiter(cfg.Security.RateLimit, cfg.Security.RateLimitWindow)

	return &Router{
		config:  cfg,
		handler: handler,
		limiter: limiter,
	}
}

// SetupRoutes configures all API routes and returns the HTTP handler
func (rt *Router) SetupRoutes() http.Handler {
	mux := http.NewServeMux()

	// API v1 routes
	mux.HandleFunc("/api/v1/health", rt.handler.Health)
	mux.HandleFunc("/api/v1/reports", rt.handler.GetReports)
	mux.HandleFunc("/api/v1/reports/", rt.handler.GetReportByID)
	mux.HandleFunc("/api/v1/buses/status", rt.handler.GetBusStatus)
	mux.HandleFunc("/api/v1/analytics", rt.handler.GetAnalytics)

	mux.HandleFunc("/api/v1/processor/status", rt.handler.GetProcessorStatus)

	// Legacy routes for backward compatibility
	mux.HandleFunc("/health", rt.handler.Health)
	mux.HandleFunc("/reports", rt.handler.GetReports)

	// Apply middleware chain
	var handler http.Handler = mux

	// Recovery middleware (innermost)
	handler = middleware.Recovery()(handler)

	// Request logging middleware
	handler = middleware.RequestLogger()(handler)

	// Authentication middleware - DISABLED FOR NOW
	// handler = middleware.Authenticate(rt.config.Security.APIKey)(handler)

	// Rate limiting middleware
	handler = rt.limiter.RateLimit()(handler)

	// Security headers middleware
	handler = middleware.SecurityHeaders()(handler)

	// CORS middleware (outermost)
	handler = middleware.CORS(rt.config.CORS)(handler)

	return handler
}
