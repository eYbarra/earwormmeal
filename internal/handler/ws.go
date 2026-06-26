package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/earworm/vibesboard/internal/hub"
	"github.com/gorilla/websocket"
)

const (
	// pongWait is the maximum time to wait for a pong response from the client.
	pongWait = 60 * time.Second

	// pingInterval is how often we send pings. Must be less than pongWait.
	pingInterval = 54 * time.Second

	// writeWait is the time allowed to write a message to the client.
	writeWait = 10 * time.Second
)

// upgrader is the WebSocket upgrader with permissive origin check for development.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSHandler handles WebSocket upgrade requests and manages client connections.
type WSHandler struct {
	hub    *hub.Hub
	logger *slog.Logger
}

// NewWSHandler creates a new WSHandler with the given hub and logger.
func NewWSHandler(h *hub.Hub, logger *slog.Logger) *WSHandler {
	return &WSHandler{
		hub:    h,
		logger: logger,
	}
}

// ServeHTTP upgrades the HTTP connection to a WebSocket, registers the client
// with the hub, and starts the read and write pump goroutines.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := hub.NewClient(conn)
	h.hub.Register(client)

	go h.writePump(client)
	go h.readPump(client)
}

// readPump reads messages from the WebSocket connection.
// It detects disconnection and triggers unregister when the connection is lost.
func (h *WSHandler) readPump(client *hub.Client) {
	defer func() {
		h.hub.Unregister(client)
	}()

	client.Conn.SetReadDeadline(time.Now().Add(pongWait))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Warn("websocket read error", "error", err)
			}
			break
		}
	}
}

// writePump drains the client's send channel and writes messages to the WebSocket.
// It also sends periodic pings to keep the connection alive.
func (h *WSHandler) writePump(client *hub.Client) {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel — connection is done.
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
