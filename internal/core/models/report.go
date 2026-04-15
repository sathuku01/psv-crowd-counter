package models

import "time"

type Report struct {
	Timestamp      time.Time `json:"timestamp"`
	BusID          string    `json:"bus_id"`
	PassengerCount int       `json:"passenger_count"`
	SpeedKPH       float64   `json:"speed_kph"`
}
