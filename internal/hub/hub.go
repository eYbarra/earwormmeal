package hub

import (
	"encoding/json"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

const sendBufferSize = 256

// Client represents a single WebSocket connection tracked by the Hub.
type Client struct {
	Conn *websocket.Conn
	Send chan []byte
}

// Hub manages active WebSocket connections and broadcasts messages to all clients.
// It runs as a single goroutine processing register/unregister/broadcast via channels.
type Hub struct {
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	done       chan struct{}
	count      atomic.Int64
}

// New creates a new Hub ready to be started with Run().
func New() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		done:       make(chan struct{}),
	}
}

// Run starts the main event loop. It should be launched as a goroutine.
// It processes register, unregister, and broadcast events sequentially.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
			h.count.Store(int64(len(h.clients)))
			h.broadcastConnectedCount()

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				h.removeClient(client)
				h.broadcastConnectedCount()
			}

		case msg := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.Send <- msg:
					// Sent successfully.
				default:
					// Send channel is full — client is dead.
					h.removeClient(client)
				}
			}

		case <-h.done:
			// Shutdown: close all client connections.
			for client := range h.clients {
				h.removeClient(client)
			}
			return
		}
	}
}

// Register adds a client to the hub. Safe to call from any goroutine.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub. Safe to call from any goroutine.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast sends a message to all connected clients. Safe to call from any goroutine.
func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

// ConnectedCount returns the current number of connected clients.
// Safe to call from any goroutine (uses atomic counter).
func (h *Hub) ConnectedCount() int {
	return int(h.count.Load())
}

// Shutdown closes all client connections and stops the Run loop.
func (h *Hub) Shutdown() {
	close(h.done)
}

// removeClient closes the client's send channel and connection, then removes it from the map.
func (h *Hub) removeClient(client *Client) {
	delete(h.clients, client)
	close(client.Send)
	client.Conn.Close()
	h.count.Store(int64(len(h.clients)))
}

// broadcastConnectedCount marshals and sends a connected_count message to all clients.
func (h *Hub) broadcastConnectedCount() {
	msg, err := json.Marshal(map[string]interface{}{
		"type":    "connected_count",
		"payload": len(h.clients),
	})
	if err != nil {
		return
	}
	for client := range h.clients {
		select {
		case client.Send <- msg:
		default:
			// Dead client — remove it.
			h.removeClient(client)
		}
	}
}

// NewClient creates a new Client with a buffered send channel.
func NewClient(conn *websocket.Conn) *Client {
	return &Client{
		Conn: conn,
		Send: make(chan []byte, sendBufferSize),
	}
}
