# PSV Crowd Counter Frontend

A Go-based frontend application for the PSV Crowd Counter system, providing real-time monitoring and analytics for public service vehicle passenger counting.

## Features

- **Dashboard**: Real-time overview of passenger counts, active vehicles, and system status
- **Vehicle Management**: View and filter all vehicles with their current status
- **Analytics**: Historical data analysis with hourly/daily statistics and peak usage insights
- **Real-time Updates**: Auto-refresh every 30 seconds for live data
- **Responsive Design**: Works on desktop and mobile devices
- **Search & Filter**: Filter vehicles by status, search by vehicle ID

## Architecture

The frontend uses a server-rendered HTML approach with Go templates:

```
frontend/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── handlers/            # HTTP request handlers
│   │   └── handlers.go
│   ├── services/            # API service layer
│   │   └── api_service.go
│   ├── templates/           # HTML templates
│   │   ├── layout.html      # Base layout template
│   │   ├── dashboard.html   # Dashboard page
│   │   ├── vehicles.html    # Vehicles page
│   │   ├── analytics.html   # Analytics page
│   │   └── error.html       # Error page
│   └── models/              # Data models
│       └── models.go
├── static/
│   ├── css/
│   │   └── style.css        # Main stylesheet
│   └── js/
│       └── main.js          # Client-side JavaScript
└── README.md
```

## Prerequisites

- Go 1.24+
- Backend API running (default: http://localhost:8080)

## Installation

1. Navigate to the project root:
```bash
cd psv-crowd-counter
```

2. Install dependencies:
```bash
go mod download
```

3. Build the frontend:
```bash
go build -o frontend-server ./frontend/cmd
```

## Configuration

The frontend can be configured using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `FRONTEND_PORT` | Port for the frontend server | `3000` |
| `BACKEND_API_URL` | Backend API base URL | `http://localhost:8080` |

Example:
```bash
export FRONTEND_PORT=8080
export BACKEND_API_URL=http://localhost:9090
```

## Running the Frontend

### Option 1: Using go run
```bash
go run ./frontend/cmd
```

### Option 2: Using compiled binary
```bash
./frontend-server
```

### Option 3: With custom configuration
```bash
FRONTEND_PORT=8080 BACKEND_API_URL=http://localhost:9090 go run ./frontend/cmd
```

The frontend will be available at: http://localhost:3000

## Pages

### Dashboard (`/`)
- Total passengers count
- Active vehicles count
- Average passenger density
- Vehicle status cards
- Recent reports table
- Auto-refresh every 30 seconds

### Vehicles (`/vehicles`)
- List of all vehicles
- Status indicators (active/idle/inactive)
- Passenger counts and speed
- Search by vehicle ID
- Filter by status
- Last updated timestamps

### Analytics (`/analytics`)
- Peak usage hours
- Hourly statistics chart
- Daily statistics table
- Filter by vehicle ID and time range
- Insights and trends

## API Endpoints

The frontend exposes the following API endpoints:

- `GET /api/health` - Health check
- `GET /api/reports` - Get all reports (JSON)

## Styling

The frontend uses a custom CSS design with:
- CSS variables for easy theming
- Responsive grid layouts
- Mobile-first approach
- Smooth animations and transitions
- Status indicators with color coding

## JavaScript Features

- Health status monitoring
- Number formatting
- Timestamp formatting
- Debounced search
- Notification system
- Tooltip support

## Browser Support

- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)

## Development

### Adding New Pages

1. Create a new template in `internal/templates/`
2. Add handler in `internal/handlers/handlers.go`
3. Register route in `cmd/main.go`

### Modifying Styles

Edit `static/css/style.css`. Changes will be reflected immediately in development mode.

### Adding JavaScript Functionality

Edit `static/js/main.js`. The file exports utility functions via `window.PSVCrowdCounter`.

## Troubleshooting

### Frontend won't start
- Check if port 3000 is available
- Verify Go version (1.24+ required)
- Ensure all dependencies are installed

### Cannot connect to backend
- Verify backend is running
- Check `BACKEND_API_URL` environment variable
- Ensure backend CORS settings allow frontend requests

### Templates not loading
- Verify template files exist in `internal/templates/`
- Check file permissions
- Review server logs for errors

## Future Enhancements

- [ ] WebSocket support for real-time updates
- [ ] Map visualization for vehicle locations
- [ ] Export reports (CSV/PDF)
- [ ] Dark mode toggle
- [ ] User authentication
- [ ] Push notifications
- [ ] Advanced charting with Chart.js
- [ ] Mobile app (PWA)

## License

See the main project LICENSE file.
