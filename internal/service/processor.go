package service

import (
	"log"
	"time"

	cam "psv-crowd-counter/internal/camera"
	"psv-crowd-counter/internal/core/models"
	det "psv-crowd-counter/internal/detector"
	"psv-crowd-counter/internal/gps"
	"psv-crowd-counter/internal/storage"
)

type Processor struct {
	camera         cam.Camera
	detector       det.Detector
	gps            gps.GPS
	store          storage.Store
	busID          string
	reportInterval time.Duration
	quit           chan struct{}
}

func NewProcessor(camera cam.Camera, detector det.Detector, gps gps.GPS, store storage.Store, busID string, interval time.Duration) *Processor {
	return &Processor{camera: camera, detector: detector, gps: gps, store: store, busID: busID, reportInterval: interval, quit: make(chan struct{})}
}

func (p *Processor) Start() {
	p.camera.Start()
	p.detector.Start()
	frames := p.camera.Frames()
	results := p.detector.Process(frames)
	go p.run(results)
}

func (p *Processor) Stop() {
	close(p.quit)
	p.camera.Stop()
	p.detector.Stop()
}

func (p *Processor) run(results <-chan det.Result) {
	ticker := time.NewTicker(p.reportInterval)
	defer ticker.Stop()

	var last models.Report
	for {
		select {
		case res, ok := <-results:
			if !ok {
				return
			}
			last = models.Report{Timestamp: res.Timestamp.Timestamp, BusID: p.busID, Front: res.Front, Rear: res.Rear, SpeedKPH: p.gps.CurrentSpeedKPH()}
		case <-ticker.C:
			if last.BusID == "" {
				continue
			}
			if last.SpeedKPH > 5.0 {
				if err := p.store.Save(last); err != nil {
					log.Printf("failed to save report: %v", err)
				} else {
					log.Printf("report saved: front=%d rear=%d speed=%.1f", last.Front, last.Rear, last.SpeedKPH)
				}
			} else {
				log.Printf("speed %.1f <= 5.0, skipping report", last.SpeedKPH)
			}
		case <-p.quit:
			return
		}
	}
}

func (p *Processor) Status() map[string]interface{} {
	return map[string]interface{}{"bus_id": p.busID, "report_interval_seconds": int(p.reportInterval.Seconds())}
}
