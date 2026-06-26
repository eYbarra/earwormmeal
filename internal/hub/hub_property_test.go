package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"pgregory.net/rapid"
)

// startTestWSServer creates a single shared WebSocket test server that can accept
// multiple connections. The server keeps each connection alive until the
// client closes it. Returns the ws URL and a cleanup function.
func startTestWSServer() (string, func()) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
	return "ws" + strings.TrimPrefix(srv.URL, "http"), srv.Close
}

// dialTestConn creates a real WebSocket connection to the test server.
func dialTestConn(t *rapid.T, wsURL string) *websocket.Conn {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

// waitForCountRapid polls ConnectedCount until it equals the expected value or times out.
func waitForCountRapid(t *rapid.T, h *Hub, expected int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if h.ConnectedCount() == expected {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for ConnectedCount == %d, got %d", expected, h.ConnectedCount())
}

// Property 9: Hub connected count invariant
// For any random sequence of connect and disconnect operations,
// ConnectedCount() always equals connects minus disconnects.
// **Validates: Requirements 6.2, 6.3**
func TestProperty9_HubConnectedCountInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		wsURL, closeServer := startTestWSServer()
		defer closeServer()
		h := New()
		go h.Run()
		defer h.Shutdown()

		// Generate a random number of clients to register (1-10).
		numClients := rapid.IntRange(1, 10).Draw(t, "numClients")

		// Create and register all clients.
		clients := make([]*Client, numClients)
		for i := range numClients {
			conn := dialTestConn(t, wsURL)
			clients[i] = NewClient(conn)
			h.Register(clients[i])
		}
		waitForCountRapid(t, h, numClients, 2*time.Second)

		// Verify count equals number of registered clients.
		if got := h.ConnectedCount(); got != numClients {
			t.Fatalf("after %d registers, expected ConnectedCount == %d, got %d", numClients, numClients, got)
		}

		// Generate a random subset to disconnect.
		numDisconnects := rapid.IntRange(0, numClients).Draw(t, "numDisconnects")
		for i := range numDisconnects {
			h.Unregister(clients[i])
		}
		expected := numClients - numDisconnects
		waitForCountRapid(t, h, expected, 2*time.Second)

		// Verify invariant: ConnectedCount == connects - disconnects.
		if got := h.ConnectedCount(); got != expected {
			t.Fatalf("after %d registers and %d unregisters, expected ConnectedCount == %d, got %d",
				numClients, numDisconnects, expected, got)
		}
	})
}

// Property 10: Hub broadcasts accurate connected count
// For any connect or disconnect event, the Hub broadcasts a "connected_count"
// message whose payload equals the current ConnectedCount() after the operation.
// **Validates: Requirements 6.4**
func TestProperty10_HubBroadcastsAccurateConnectedCount(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		wsURL, closeServer := startTestWSServer()
		defer closeServer()
		h := New()
		go h.Run()
		defer h.Shutdown()

		// Generate a random number of clients (2-8) so we can observe broadcasts.
		numClients := rapid.IntRange(2, 8).Draw(t, "numClients")

		// Register clients one by one and verify the broadcast after each registration.
		clients := make([]*Client, numClients)
		for i := range numClients {
			conn := dialTestConn(t, wsURL)
			clients[i] = NewClient(conn)
			h.Register(clients[i])
			waitForCountRapid(t, h, i+1, 2*time.Second)

			// After registering client i, all connected clients (0..i) should
			// receive a connected_count broadcast with payload == i+1.
			expectedCount := i + 1
			for j := 0; j <= i; j++ {
				msg := readFromSendRapid(t, clients[j], 2*time.Second)
				verifyConnectedCountPayload(t, msg, expectedCount)
			}
		}

		// Now disconnect a random subset and verify broadcast after each.
		numDisconnects := rapid.IntRange(1, numClients-1).Draw(t, "numDisconnects")
		for i := range numDisconnects {
			h.Unregister(clients[i])
			expectedCount := numClients - (i + 1)
			waitForCountRapid(t, h, expectedCount, 2*time.Second)

			// All remaining connected clients should receive updated count.
			for j := numDisconnects; j < numClients; j++ {
				// Only clients that haven't been unregistered yet get broadcasts.
				if j > i {
					msg := readFromSendRapid(t, clients[j], 2*time.Second)
					verifyConnectedCountPayload(t, msg, expectedCount)
				}
			}
		}
	})
}

// readFromSendRapid reads one message from the client's Send channel with a timeout.
func readFromSendRapid(t *rapid.T, c *Client, timeout time.Duration) []byte {
	select {
	case msg, ok := <-c.Send:
		if !ok {
			t.Fatal("Send channel closed unexpectedly")
		}
		return msg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for message on Send channel")
		return nil
	}
}

// verifyConnectedCountPayload checks that a raw JSON message is a connected_count
// broadcast with the expected payload.
func verifyConnectedCountPayload(t *rapid.T, raw []byte, expectedCount int) {
	var msg struct {
		Type    string `json:"type"`
		Payload int    `json:"payload"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("failed to unmarshal broadcast: %v (raw: %s)", err, raw)
	}
	if msg.Type != "connected_count" {
		t.Fatalf("expected message type 'connected_count', got %q", msg.Type)
	}
	if msg.Payload != expectedCount {
		t.Fatalf("expected connected_count payload %d, got %d", expectedCount, msg.Payload)
	}
}
