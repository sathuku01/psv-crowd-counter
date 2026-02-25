bus-monitoring-system/
├── cmd/
│   └── bus-monitor/
│       └── main.go              # Single entry point
├── internal/
│   ├── camera/
│   │   ├── camera.go            # Camera capture
│   │   └── frame.go             # Frame handling
│   ├── detector/
│   │   ├── yolo.go              # Person detection
│   │   └── face.go              # Face/eye detection
│   ├── counter/
│   │   └── counter.go            # Crowd counting logic
│   ├── alert/
│   │   └── alert.go              # Drowsiness alerts
│   ├── storage/
│   │   └── store.go              # Save reports locally
│   ├── sender/
│   │   └── sender.go             # Send to cloud
│   └── config/
│       └── config.go              # Settings
├── models/
│   ├── bus.go                     # Bus info
│   ├── person.go                  # Person detection
│   └── report.go                  # Crowd report
├── configs/
│   └── config.yaml                 # Configuration
├── scripts/
│   └── run.sh                      # Start script
├── go.mod
└── README.md