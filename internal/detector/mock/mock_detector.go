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
			front := m.rnd.Intn(40)
			rear := m.rnd.Intn(40)
			select {
			case m.out <- detector.Result{Timestamp: f, Front: front, Rear: rear}:
			case <-m.quit:
				return
			}
		}
		close(m.out)
	}()
	return m.out
}
