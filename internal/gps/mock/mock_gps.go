package mock

import "sync/atomic"

type MockGPS struct {
	speed atomic.Value
}

func NewMockGPS(initial float64) *MockGPS {
	g := &MockGPS{}
	g.speed.Store(initial)
	return g
}

func (g *MockGPS) SetSpeed(v float64) { g.speed.Store(v) }

func (g *MockGPS) CurrentSpeedKPH() float64 {
	v := g.speed.Load()
	if v == nil {
		return 0
	}
	return v.(float64)
}
