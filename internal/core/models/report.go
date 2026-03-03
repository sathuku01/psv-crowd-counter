package models

import "time"

type Report struct {
	Timestamp time.Time `json:"timestamp"`
	BusID     string    `json:"bus_id"`
	Front     int       `json:"front_count"`
	Rear      int       `json:"rear_count"`
	SpeedKPH  float64   `json:"speed_kph"`
}
