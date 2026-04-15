package mock

import (
	"math/rand"
	"time"

	"psv-crowd-counter/internal/camera"
	"psv-crowd-counter/internal/detector"
)

type MockDetector struct {
	out  chan detector.Result
	quit chan struct{}
	rnd  *rand.Rand
}

func NewMockDetector() *MockDetector {
	return &MockDetector{out: make(chan detector.Result, 10), quit: make(chan struct{}), rnd: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (m *MockDetector) Start() {}
func (m *MockDetector) Stop()  { close(m.quit); close(m.out) }

func (m *MockDetector) Process(in <-chan camera.Frame) <-chan detector.Result {
	go func() {
		for f := range in {
			count := m.rnd.Intn(40)
			// Generate mock bounding boxes for frontend display
			boxes := make([]detector.Box, count)
			for i := 0; i < count; i++ {
				boxes[i] = detector.Box{
					X1:   m.rnd.Intn(640),
					Y1:   m.rnd.Intn(480),
					X2:   m.rnd.Intn(100) + 50,
					Y2:   m.rnd.Intn(100) + 50,
					Type: "person",
				}
			}
			select {
			case m.out <- detector.Result{Timestamp: f, Count: count, Boxes: boxes}:
			case <-m.quit:
				return
			}
		}
		close(m.out)
	}()
	return m.out
}
