package camera

import "time"

type Frame struct {
	Timestamp time.Time
	Payload   []byte
}

type Camera interface {
	Start()
	Stop()
	Frames() <-chan Frame
}
