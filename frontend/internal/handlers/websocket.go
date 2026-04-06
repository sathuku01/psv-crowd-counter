package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"psv-crowd-counter/frontend/internal/services"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// WebSocketHandler handles WebSocket connections for real-time video processing
type WebSocketHandler struct {
	apiService *services.APIService
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(apiService *services.APIService) *WebSocketHandler {
	return &WebSocketHandler{
		apiService: apiService,
	}
}

// HandleWebSocket handles the WebSocket connection
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get mode from query parameter (sleep or crowd)
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "sleep"
	}

	// Get the target URL based on mode
	var targetURL string
	if mode == "crowd" {
		targetURL = h.apiService.GetCrowdDetectURL()
	} else {
		targetURL = h.apiService.GetMediaPipeURL()
	}

	// Upgrade to WebSocket
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	})
	if err != nil {
		log.Printf("WebSocket accept error: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closing")

	log.Printf("WebSocket connected: mode=%s, target=%s", mode, targetURL)

	// Create a context with timeout for operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Handle incoming messages from client
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read message from client (video frame)
			_, msg, err := conn.Reader(ctx)
			if err != nil {
				log.Printf("WebSocket read error: %v", err)
				return
			}

			// Read the message body
			frameData, err := io.ReadAll(msg)
			if err != nil {
				log.Printf("WebSocket read body error: %v", err)
				continue
			}

			// Forward to backend and get response
			result, err := h.processFrame(ctx, targetURL, frameData)
			if err != nil {
				// Send error back to client
				errorMsg := map[string]interface{}{
					"error": err.Error(),
				}
				if jsonErr := wsjson.Write(ctx, conn, errorMsg); jsonErr != nil {
					log.Printf("WebSocket write error: %v", jsonErr)
				}
				continue
			}

			// Send result back to client
			if err := wsjson.Write(ctx, conn, result); err != nil {
				log.Printf("WebSocket write error: %v", err)
			}
		}
	}
}

// processFrame sends the frame to the backend and returns the result
func (h *WebSocketHandler) processFrame(ctx context.Context, targetURL string, frameData []byte) (map[string]interface{}, error) {
	// Create HTTP request to forward the frame
	url := targetURL + "/detect"
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Use the frame data as-is (should be JSON with image field)
	req.Body = io.NopCloser(stringToReader(string(frameData)))

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// stringToReader creates a reader from string
func stringToReader(s string) io.Reader {
	return &stringReader{data: s}
}

type stringReader struct {
	data string
	pos  int
}

func (r *stringReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
