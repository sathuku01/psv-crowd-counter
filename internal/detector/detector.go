package detector

import "psv-crowd-counter/internal/camera"

type Result struct {
	Timestamp camera.Frame
	Front     int
	Rear      int
}

type Detector interface {
	Start()
	Stop()
	Process(in <-chan camera.Frame) <-chan Result
}
