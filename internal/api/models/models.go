package models

import (
	"time"
)

// APIResponse is a standard API response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// APIError represents an API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Meta contains metadata for paginated responses
type Meta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
}

// APIReport represents a crowd counting report for API responses
type APIReport struct {
	ID         string    `json:"id"`
	BusID      string    `json:"bus_id"`
	Front      int       `json:"front_count"`
	Rear       int       `json:"rear_count"`
	Passengers int       `json:"total_passengers"`
	Timestamp  time.Time `json:"timestamp"`
}

// ReportRequest represents a request to create a report
type ReportRequest struct {
	BusID string `json:"bus_id" validate:"required"`
	Front int    `json:"front_count" validate:"required,min=0"`
	Rear  int    `json:"rear_count" validate:"required,min=0"`
}

// ReportFilter represents filters for querying reports
type ReportFilter struct {
	BusID     string    `json:"bus_id,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Page      int       `json:"page"`
	PerPage   int       `json:"per_page"`
}

// BusStatus represents the current status of a bus
type BusStatus struct {
	BusID         string    `json:"bus_id"`
	Passengers    int       `json:"passengers"`
	LastUpdated   time.Time `json:"last_updated"`
	IsActive      bool      `json:"is_active"`
	OccupancyRate float64   `json:"occupancy_rate"`
}

// Analytics represents analytics data
type Analytics struct {
	TotalReports       int         `json:"total_reports"`
	AveragePassengers  float64     `json:"average_passengers"`
	PeakHour           int         `json:"peak_hour"`
	BusStats           []BusStat   `json:"bus_stats"`
	HourlyDistribution map[int]int `json:"hourly_distribution"`
	// Driver drowsiness fields
	EyeClosureAlerts  int     `json:"eye_closure_alerts"`
	YawningAlerts     int     `json:"yawning_alerts"`
	HeadPoseAlerts    int     `json:"head_pose_alerts"`
	CriticalAlerts    int     `json:"critical_alerts"`
	AverageEAR        float64 `json:"average_ear"`
	AverageMAR        float64 `json:"average_mar"`
	DetectionAccuracy float64 `json:"detection_accuracy"`
	ActiveSessions    int     `json:"active_sessions"`
}

// BusStat represents statistics for a specific bus
type BusStat struct {
	BusID             string  `json:"bus_id"`
	TotalReports      int     `json:"total_reports"`
	AveragePassengers float64 `json:"average_passengers"`
	MaxPassengers     int     `json:"max_passengers"`
}

// TokenRequest represents a request for authentication token
type TokenRequest struct {
	APIKey string `json:"api_key" validate:"required"`
}

// TokenResponse represents a successful token response
type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	TokenType string    `json:"token_type"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Items interface{} `json:"items"`
	Meta  Meta        `json:"meta"`
}
