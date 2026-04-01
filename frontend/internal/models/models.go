package models

import "time"

// Report represents a crowd count report from the backend
type Report struct {
	Timestamp time.Time `json:"timestamp"`
	BusID     string    `json:"bus_id"`
	Front     int       `json:"front"`
	Rear      int       `json:"rear"`
	SpeedKPH  float64   `json:"speed_kph"`
}

// VehicleStatus represents the current status of a vehicle
type VehicleStatus struct {
	BusID          string    `json:"bus_id"`
	PassengerCount int       `json:"passenger_count"`
	SpeedKPH       float64   `json:"speed_kph"`
	LastUpdated    time.Time `json:"last_updated"`
	Status         string    `json:"status"` // "active", "inactive", "idle"
}

// DashboardData represents the data for the dashboard view
type DashboardData struct {
	TotalPassengers int             `json:"total_passengers"`
	ActiveVehicles  int             `json:"active_vehicles"`
	TotalVehicles   int             `json:"total_vehicles"`
	AverageDensity  float64         `json:"average_density"`
	Vehicles        []VehicleStatus `json:"vehicles"`
	RecentReports   []Report        `json:"recent_reports"`
	LastUpdated     time.Time       `json:"last_updated"`
}

// AnalyticsData represents historical analytics data
type AnalyticsData struct {
	HourlyData []HourlyStats `json:"hourly_data"`
	DailyData  []DailyStats  `json:"daily_data"`
	PeakHours  []PeakHour    `json:"peak_hours"`
}

// HourlyStats represents statistics for an hour
type HourlyStats struct {
	Hour         int     `json:"hour"`
	AverageCount float64 `json:"average_count"`
	MaxCount     int     `json:"max_count"`
	ReportCount  int     `json:"report_count"`
}

// DailyStats represents statistics for a day
type DailyStats struct {
	Date         string  `json:"date"`
	AverageCount float64 `json:"average_count"`
	MaxCount     int     `json:"max_count"`
	TotalReports int     `json:"total_reports"`
}

// PeakHour represents a peak usage hour
type PeakHour struct {
	Hour         int     `json:"hour"`
	AverageCount float64 `json:"average_count"`
	Day          string  `json:"day"`
}

// FilterParams represents filtering parameters for queries
type FilterParams struct {
	Route     string `json:"route"`
	VehicleID string `json:"vehicle_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}
