package service

import (
	"log"
	"time"

	cam "psv-crowd-counter/internal/camera"
	"psv-crowd-counter/internal/core/models"
	det "psv-crowd-counter/internal/detector"
	"psv-crowd-counter/internal/storage"
)

type Processor struct {
	camera         cam.Camera
	detector       det.Detector
	store          storage.Store
	busID          string
	reportInterval time.Duration
	quit           chan struct{}
	detections     chan<- det.Result
}

func NewProcessor(camera cam.Camera, detector det.Detector, store storage.Store, busID string, interval time.Duration, detections chan<- det.Result) *Processor {
	return &Processor{camera: camera, detector: detector, store: store, busID: busID, reportInterval: interval, quit: make(chan struct{}), detections: detections}
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
			last = models.Report{Timestamp: res.Timestamp.Timestamp, BusID: p.busID, PassengerCount: res.Count}
			// Send detections to websocket
			if p.detections != nil {
				select {
				case p.detections <- res:
				default:
					// Drop if channel full
				}
			}
		case <-ticker.C:
			if last.BusID == "" {
				continue
			}
			if err := p.store.Save(last); err != nil {
				log.Printf("failed to save report: %v", err)
			} else {
				log.Printf("report saved: passenger_count=%d", last.PassengerCount)
			}
		case <-p.quit:
			return
		}
	}
}

func (p *Processor) Status() map[string]interface{} {
	return map[string]interface{}{"bus_id": p.busID, "report_interval_seconds": int(p.reportInterval.Seconds())}
}
