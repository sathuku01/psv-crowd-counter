package detector

import "psv-crowd-counter/internal/camera"

// Detection represents raw detection data from OpenCV
type Detection struct {
	X      int
	Y      int
	Width  int
	Height int
	Type   string // "person" or "face"
}

// Box represents a bounding box in frontend-compatible format
type Box struct {
	X1   int    `json:"x1"`
	Y1   int    `json:"y1"`
	X2   int    `json:"x2"`
	Y2   int    `json:"y2"`
	Type string `json:"type"`
}

// Result contains detection results for both backend processing and frontend display
type Result struct {
	Timestamp  camera.Frame
	Count      int         `json:"count"`
	Detections []Detection `json:"-"`     // Raw detections for internal use
	Boxes      []Box       `json:"boxes"` // Frontend-compatible boxes format
}

type Detector interface {
	Start()
	Stop()
	Process(in <-chan camera.Frame) <-chan Result
}
