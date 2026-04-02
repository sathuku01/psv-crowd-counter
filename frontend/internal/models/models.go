package models

import "time"

// Report represents a crowd count report from the backend
type Report struct {
	ID         string    `json:"id"`
	BusID      string    `json:"bus_id"`
	Front      int       `json:"front_count"`
	Rear       int       `json:"rear_count"`
	Passengers int       `json:"total_passengers"`
	Timestamp  time.Time `json:"timestamp"`
}

// VehicleStatus represents the current status of a vehicle
type VehicleStatus struct {
	BusID          string    `json:"bus_id"`
	PassengerCount int       `json:"passenger_count"`
	LastUpdated    time.Time `json:"last_updated"`
	Status         string    `json:"status"` // "active", "inactive", "idle"
}

// DashboardData represents the data for the dashboard view
type DashboardData struct {
	TotalPassengers int             `json:"total_passengers"`
	ActiveVehicles  int             `json:"active_vehicles"`
	TotalVehicles   int             `json:"total_vehicles"`
	Vehicles        []VehicleStatus `json:"vehicles"`
	RecentReports   []Report        `json:"recent_reports"`
	LastUpdated     time.Time       `json:"last_updated"`
}

// AnalyticsData represents the data for the analytics view
type AnalyticsData struct {
	TotalPassengers    int         `json:"total_passengers"`
	ActiveVehicles     int         `json:"active_vehicles"`
	TotalVehicles      int         `json:"total_vehicles"`
	TotalReports       int         `json:"total_reports"`
	AveragePassengers  float64     `json:"average_passengers"`
	PeakHour           int         `json:"peak_hour"`
	FrontDoorCount     int         `json:"front_door_count"`
	RearDoorCount      int         `json:"rear_door_count"`
	HourlyDistribution map[int]int `json:"hourly_distribution"`
	MaxHourlyCount     int         `json:"max_hourly_count"`
	BusStats           []BusStat   `json:"bus_stats"`
	RecentReports      []Report    `json:"recent_reports"`
	LastUpdated        time.Time   `json:"last_updated"`
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
