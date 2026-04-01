package models

import (
	"image"
	"time"
)

// Detection represents a single object detection result
type Detection struct {
	Box   image.Rectangle
	Score float32
	Class int
}

// HeadDetection extends Detection with tracking information for crowd counting
type HeadDetection struct {
	Detection
	ID       int
	Centroid image.Point
	Age      int
	LastSeen time.Time
}

// CrowdCountResult holds the results of crowd counting analysis
type CrowdCountResult struct {
	Timestamp     time.Time
	HeadCount     int
	PersonCount   int
	TrackedHeads  []HeadDetection
	PersonDetects []Detection
	HeadDetects   []Detection
}
