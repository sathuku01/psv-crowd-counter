package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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

	// Use background context for the connection (no timeout on the connection itself)
	ctx := context.Background()

	// Handle incoming messages from client
	for {
		// Read message from client (video frame)
		_, msg, err := conn.Reader(ctx)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			// Check if it's a normal close
			if err == io.EOF || strings.Contains(err.Error(), "close") {
				log.Printf("WebSocket closed by client")
			} else {
				log.Printf("WebSocket error: %v", err)
			}
			return
		}

		// Read the message body
		frameData, err := io.ReadAll(msg)
		if err != nil {
			log.Printf("WebSocket read body error: %v", err)
			continue
		}

		// Check if it's a ping message
		var pingCheck map[string]interface{}
		if err := json.Unmarshal(frameData, &pingCheck); err == nil {
			if pingType, ok := pingCheck["type"].(string); ok && pingType == "ping" {
				// Respond with pong
				if err := wsjson.Write(ctx, conn, map[string]interface{}{"type": "pong"}); err != nil {
					log.Printf("WebSocket write pong error: %v", err)
				}
				log.Printf("Received ping, sent pong")
				continue
			}
		}

		// Create a timeout context for processing each frame
		procCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

		// Forward to backend and get response
		result, err := h.processFrame(procCtx, targetURL, frameData)
		if err != nil {
			// Send error back to client
			errorMsg := map[string]interface{}{
				"error": err.Error(),
			}
			if jsonErr := wsjson.Write(ctx, conn, errorMsg); jsonErr != nil {
				log.Printf("WebSocket write error: %v", jsonErr)
			}
			cancel()
			continue
		}

		// Send result back to client
		if err := wsjson.Write(ctx, conn, result); err != nil {
			log.Printf("WebSocket write error: %v", err)
		}

		cancel()
	}
}

// processFrame sends the frame to the backend and returns the result
func (h *WebSocketHandler) processFrame(ctx context.Context, targetURL string, frameData []byte) (map[string]interface{}, error) {
	// Parse the incoming JSON from client
	var clientData struct {
		Image string `json:"image"`
		Mode  string `json:"mode"`
	}

	if err := json.Unmarshal(frameData, &clientData); err != nil {
		log.Printf("Failed to parse client frame data: %v", err)
		return nil, fmt.Errorf("invalid frame data format: %w", err)
	}

	// Create JSON payload for backend
	payload, err := json.Marshal(map[string]string{
		"image": clientData.Image,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create payload: %w", err)
	}

	// Create HTTP request to forward the frame
	url := targetURL + "/detect"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

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
