# Backend API Documentation

## Overview

The PSV Crowd Counter Backend API provides secure endpoints for managing crowd counting reports, bus status, and analytics data. The API follows RESTful conventions and implements comprehensive security measures.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

All API endpoints (except `/health`) require authentication via API key. Include the API key in one of the following ways:

### Header (Recommended)
```
X-API-Key: your-api-key-here
```

### Authorization Header
```
Authorization: Bearer your-api-key-here
```

## Security Features

- **API Key Authentication**: All endpoints require valid API key
- **Rate Limiting**: 100 requests per minute per client (configurable)
- **CORS**: Configurable cross-origin resource sharing
- **Security Headers**: XSS protection, content type options, frame options
- **HTTPS Support**: Optional TLS encryption
- **Input Validation**: All inputs are validated and sanitized
- **Request Logging**: All requests are logged for audit purposes

## Endpoints

### Health Check

Check API health status (no authentication required).

**Endpoint**: `GET /api/v1/health`

**Response**:
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "timestamp": "2024-01-01T12:00:00Z",
    "version": "1.0.0",
    "uptime": "2h30m15s"
  }
}
```

### Reports

#### Get All Reports

Retrieve all crowd counting reports with optional filtering and pagination.

**Endpoint**: `GET /api/v1/reports`

**Query Parameters**:
- `bus_id` (string): Filter by bus ID
- `start_time` (RFC3339): Filter reports after this time
- `end_time` (RFC3339): Filter reports before this time
- `min_speed` (float): Minimum speed in km/h
- `max_speed` (float): Maximum speed in km/h
- `page` (int): Page number (default: 1)
- `per_page` (int): Items per page (default: 20, max: 100)

**Example Request**:
```
GET /api/v1/reports?bus_id=BUS-001&page=1&per_page=10
```

**Response**:
```json
{
  "success": true,
  "data": [
    {
      "id": "BUS-001-1704110400000000000",
      "bus_id": "BUS-001",
      "front_count": 15,
      "rear_count": 12,
      "total_passengers": 27,
      "speed_kph": 25.5,
      "timestamp": "2024-01-01T12:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 10,
    "total": 100,
    "total_pages": 10
  }
}
```

#### Get Report by ID

Retrieve a specific report by its ID.

**Endpoint**: `GET /api/v1/reports/{id}`

**Path Parameters**:
- `id` (string): Report ID

**Example Request**:
```
GET /api/v1/reports/BUS-001-1704110400000000000
```

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "BUS-001-1704110400000000000",
    "bus_id": "BUS-001",
    "front_count": 15,
    "rear_count": 12,
    "total_passengers": 27,
    "speed_kph": 25.5,
    "timestamp": "2024-01-01T12:00:00Z"
  }
}
```

#### Create Report

Create a new crowd counting report.

**Endpoint**: `POST /api/v1/reports`

**Request Body**:
```json
{
  "bus_id": "BUS-001",
  "front_count": 15,
  "rear_count": 12,
  "speed_kph": 25.5
}
```

**Validation Rules**:
- `bus_id`: Required, non-empty string
- `front_count`: Required, non-negative integer
- `rear_count`: Required, non-negative integer
- `speed_kph`: Required, non-negative number

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "BUS-001-1704110400000000000",
    "bus_id": "BUS-001",
    "front_count": 15,
    "rear_count": 12,
    "total_passengers": 27,
    "speed_kph": 25.5,
    "timestamp": "2024-01-01T12:00:00Z"
  }
}
```

### Bus Status

Get current status of all buses.

**Endpoint**: `GET /api/v1/buses/status`

**Response**:
```json
{
  "success": true,
  "data": [
    {
      "bus_id": "BUS-001",
      "passengers": 27,
      "speed_kph": 25.5,
      "last_updated": "2024-01-01T12:00:00Z",
      "is_active": true,
      "occupancy_rate": 0.54
    }
  ]
}
```

### Analytics

Get analytics and statistics.

**Endpoint**: `GET /api/v1/analytics`

**Response**:
```json
{
  "success": true,
  "data": {
    "total_reports": 1000,
    "average_passengers": 25.5,
    "average_speed": 28.3,
    "peak_hour": 8,
    "bus_stats": [
      {
        "bus_id": "BUS-001",
        "total_reports": 100,
        "average_passengers": 27.5,
        "max_passengers": 45,
        "average_speed": 26.8
      }
    ],
    "hourly_distribution": {
      "0": 10,
      "1": 15,
      "2": 20
    }
  }
}
```

### GPS Data

Get current GPS data.

**Endpoint**: `GET /api/v1/gps`

**Response**:
```json
{
  "success": true,
  "data": {
    "speed_kph": 25.5,
    "timestamp": "2024-01-01T12:00:00Z"
  }
}
```

### Processor Status

Get processor status.

**Endpoint**: `GET /api/v1/processor/status`

**Response**:
```json
{
  "success": true,
  "data": {
    "status": "running",
    "timestamp": "2024-01-01T12:00:00Z"
  }
}
```

## Error Handling

All errors follow a consistent format:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": "Additional error details"
  }
}
```

### Common Error Codes

- `UNAUTHORIZED`: Missing or invalid API key
- `METHOD_NOT_ALLOWED`: HTTP method not supported
- `NOT_FOUND`: Resource not found
- `VALIDATION_ERROR`: Input validation failed
- `INVALID_JSON`: Malformed JSON payload
- `INTERNAL_ERROR`: Server error
- `RATE_LIMIT_EXCEEDED`: Too many requests

### HTTP Status Codes

- `200 OK`: Successful request
- `201 Created`: Resource created successfully
- `400 Bad Request`: Invalid request
- `401 Unauthorized`: Authentication failed
- `404 Not Found`: Resource not found
- `405 Method Not Allowed`: Invalid HTTP method
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error

## Rate Limiting

The API implements rate limiting to prevent abuse:

- **Default Limit**: 100 requests per minute per client
- **Client Identification**: By API key or IP address
- **Headers**: 
  - `X-RateLimit-Limit`: Maximum requests per window
  - `X-RateLimit-Remaining`: Remaining requests in current window
  - `Retry-After`: Seconds to wait when rate limited

## Configuration

The API can be configured via environment variables:

### Server Configuration
- `BACKEND_PORT`: Server port (default: 8080)
- `READ_TIMEOUT`: Read timeout (default: 15s)
- `WRITE_TIMEOUT`: Write timeout (default: 15s)
- `IDLE_TIMEOUT`: Idle timeout (default: 60s)

### Security Configuration
- `API_KEY`: API key for authentication
- `RATE_LIMIT`: Requests per minute (default: 100)
- `RATE_LIMIT_WINDOW`: Rate limit window (default: 1m)
- `ENABLE_HTTPS`: Enable HTTPS (default: false)
- `TLS_CERT_FILE`: Path to TLS certificate
- `TLS_KEY_FILE`: Path to TLS private key
- `JWT_SECRET`: JWT secret for token generation
- `TOKEN_EXPIRATION`: Token expiration time (default: 24h)

### CORS Configuration
- `CORS_ALLOWED_ORIGINS`: Comma-separated allowed origins
- `CORS_ALLOWED_METHODS`: Comma-separated allowed methods
- `CORS_ALLOWED_HEADERS`: Comma-separated allowed headers
- `CORS_ALLOW_CREDENTIALS`: Allow credentials (default: true)
- `CORS_MAX_AGE`: Preflight cache duration (default: 86400)

### Application Configuration
- `BUS_ID`: Bus identifier (default: BUS-001)
- `DATA_DIR`: Data directory path (default: data)

## Examples

### Using cURL

#### Get Health Status
```bash
curl http://localhost:8080/api/v1/health
```

#### Get Reports with Authentication
```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/reports
```

#### Create Report
```bash
curl -X POST \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"bus_id":"BUS-001","front_count":15,"rear_count":12,"speed_kph":25.5}' \
  http://localhost:8080/api/v1/reports
```

#### Get Bus Status
```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/buses/status
```

### Using JavaScript/Fetch

```javascript
const API_KEY = 'your-api-key';
const BASE_URL = 'http://localhost:8080/api/v1';

// Get reports
const response = await fetch(`${BASE_URL}/reports`, {
  headers: {
    'X-API-Key': API_KEY
  }
});
const data = await response.json();

// Create report
const newReport = await fetch(`${BASE_URL}/reports`, {
  method: 'POST',
  headers: {
    'X-API-Key': API_KEY,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    bus_id: 'BUS-001',
    front_count: 15,
    rear_count: 12,
    speed_kph: 25.5
  })
});
```

## Best Practices

1. **Always use HTTPS in production**: Set `ENABLE_HTTPS=true` and provide TLS certificates
2. **Use strong API keys**: Generate cryptographically secure API keys
3. **Implement proper error handling**: Check the `success` field in responses
4. **Respect rate limits**: Monitor rate limit headers and implement backoff
5. **Validate inputs**: Always validate data before sending to the API
6. **Log requests**: Enable request logging for debugging and audit
7. **Use pagination**: For large datasets, use pagination parameters
8. **Cache responses**: Cache frequently accessed data to reduce API calls

## Troubleshooting

### Authentication Failed
- Verify API key is correct
- Check that API key is included in `X-API-Key` or `Authorization` header
- Ensure API key hasn't been changed

### Rate Limit Exceeded
- Check `X-RateLimit-Remaining` header
- Implement exponential backoff
- Consider increasing rate limit if needed

### Validation Errors
- Check request body format
- Verify all required fields are present
- Ensure numeric values are within valid ranges

### Connection Refused
- Verify server is running
- Check that port is correct
- Ensure firewall allows connections
