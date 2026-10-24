package events

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow dashboard origins to connect
		return true
	},
}

// WSServer serves real-time events over WebSockets.
type WSServer struct {
	httpServer *http.Server
	hub        *Hub
	port       int
}

// NewWSServer initializes the WebSocket server on the given port.
func NewWSServer(hub *Hub, port int) *WSServer {
	if port <= 0 {
		port = 9091
	}

	ws := &WSServer{
		hub:  hub,
		port: port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", ws.handleWebSocket)

	ws.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return ws
}

// Start listens and serves WebSocket connections.
func (ws *WSServer) Start() error {
	return ws.httpServer.ListenAndServe()
}

// Shutdown gracefully closes the WebSocket server and its connections.
func (ws *WSServer) Shutdown(ctx context.Context) error {
	return ws.httpServer.Shutdown(ctx)
}

func (ws *WSServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	eventChan, unsubscribe := ws.hub.Subscribe(128)
	defer unsubscribe()

	// Ticker for WebSocket ping control frames
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-clientDone:
			return
		case <-pingTicker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		}
	}
}
