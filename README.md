# psv-crowd-counter

```markdown
# Bus Monitoring System

A real-time edge AI system that counts passengers and detects driver drowsiness on public buses. Built in Go

## Features

- **Crowd Counting**: Detects people, classifies as seated or standing using pose estimation
- **Drowsiness Detection**: Monitors driver's eyes via face detection, alerts if fatigue detected
- **1-Minute Reports**: Sends crowd data to cloud every minute (when speed >5km/h)
- **Offline Capable**: Stores reports locally in SQLite when no internet, retries later
- **Health API**: Local HTTP endpoints for status and metrics
- **Dual Camera Support**: Front camera for crowd, rear camera for additional coverage

## Hardware Requirements

- **Edge Device**: Raspberry Pi 4/5 + Coral/Hailo accelerator OR Kneo Pi (integrated NPU)
- **Cameras**: 2 USB or RTSP cameras (front facing crowd, rear facing, driver face)
- **GPS**: USB GPS module or CAN bus reader for speed/location
- **Connectivity**: 4G/LTE dongle or WiFi for cloud reporting
- **Storage**: 16GB+ SD card or SSD for local database

## Software Requirements

- Go 1.24+
- TensorFlow Lite Runtime
- OpenCV 4.x
- SQLite3
- Linux (Raspberry Pi OS / Ubuntu)

## Quick Start (5 Minutes)

### 1. Install Dependencies

```bash
# Clone repository
git clone https://github.com/yourcompany/psv-crowd-counter
cd psv-crowd-counter

# Install Go dependencies
go mod download

# Install system dependencies (Raspberry Pi)
sudo apt-get update
sudo apt-get install -y libopencv-dev libtensorflow-lite-dev sqlite3