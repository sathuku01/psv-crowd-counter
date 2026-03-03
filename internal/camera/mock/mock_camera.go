package mock

import (
	"time"

	"psv-crowd-counter/internal/camera"
)

type MockCamera struct {
	interval time.Duration
	frames   chan camera.Frame
	quit     chan struct{}
}

func NewMockCamera(interval time.Duration) *MockCamera {
	return &MockCamera{interval: interval, frames: make(chan camera.Frame, 10), quit: make(chan struct{})}
}

func (m *MockCamera) Start() {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case t := <-ticker.C:
				m.frames <- camera.Frame{Timestamp: t, Payload: nil}
			case <-m.quit:
				close(m.frames)
				return
			}
		}
	}()
}

func (m *MockCamera) Stop()                       { close(m.quit) }
func (m *MockCamera) Frames() <-chan camera.Frame { return m.frames }
