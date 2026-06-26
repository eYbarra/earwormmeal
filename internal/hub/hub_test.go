package hub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestConn creates a real WebSocket connection backed by an httptest server.
func newTestConn(t *testing.T) *websocket.Conn {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Keep the server-side connection open until the test ends.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// waitForCount polls ConnectedCount until it equals the expected value or times out.
func waitForCount(t *testing.T, h *Hub, expected int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if h.ConnectedCount() == expected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for ConnectedCount == %d, got %d", expected, h.ConnectedCount())
}

func TestRegisterIncreasesConnectedCount(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Shutdown()

	conn := newTestConn(t)
	client := NewClient(conn)

	h.Register(client)
	waitForCount(t, h, 1, 500*time.Millisecond)

	if got := h.ConnectedCount(); got != 1 {
		t.Fatalf("expected ConnectedCount == 1, got %d", got)
	}
}

func TestUnregisterDecreasesConnectedCount(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Shutdown()

	conn1 := newTestConn(t)
	conn2 := newTestConn(t)
	c1 := NewClient(conn1)
	c2 := NewClient(conn2)

	h.Register(c1)
	h.Register(c2)
	waitForCount(t, h, 2, 500*time.Millisecond)

	h.Unregister(c1)
	waitForCount(t, h, 1, 500*time.Millisecond)

	if got := h.ConnectedCount(); got != 1 {
		t.Fatalf("expected ConnectedCount == 1 after unregister, got %d", got)
	}
}

func TestBroadcastDeliversToAllClients(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Shutdown()

	conn1 := newTestConn(t)
	conn2 := newTestConn(t)
	c1 := NewClient(conn1)
	c2 := NewClient(conn2)

	h.Register(c1)
	h.Register(c2)
	waitForCount(t, h, 2, 500*time.Millisecond)

	// Drain the connected_count messages that were sent on register.
	drainSendChannel(t, c1)
	drainSendChannel(t, c2)

	msg := []byte(`{"type":"new_vibe","payload":"test"}`)
	h.Broadcast(msg)

	// Both clients should receive the broadcast message on their Send channel.
	m1 := readFromSend(t, c1, 500*time.Millisecond)
	m2 := readFromSend(t, c2, 500*time.Millisecond)

	if string(m1) != string(msg) {
		t.Fatalf("client1: expected %q, got %q", msg, m1)
	}
	if string(m2) != string(msg) {
		t.Fatalf("client2: expected %q, got %q", msg, m2)
	}
}

func TestDeadClientRemovedOnBroadcast(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Shutdown()

	conn1 := newTestConn(t)
	conn2 := newTestConn(t)
	c1 := NewClient(conn1) // This client will become "dead"
	c2 := NewClient(conn2) // This client stays healthy

	h.Register(c1)
	h.Register(c2)
	waitForCount(t, h, 2, 500*time.Millisecond)

	// Brief pause to let hub finish broadcasting connected_count messages.
	time.Sleep(50 * time.Millisecond)

	// Drain any connected_count messages already in the buffer.
	drainSendChannel(t, c1)
	drainSendChannel(t, c2)

	// Fill c1's send buffer completely to simulate a dead/slow client.
	for i := 0; i < sendBufferSize; i++ {
		c1.Send <- []byte("filler")
	}

	// Now broadcast a message. Since c1's buffer is full, the hub should remove it.
	h.Broadcast([]byte(`{"type":"test"}`))

	// Wait for the hub to process the broadcast and remove the dead client.
	waitForCount(t, h, 1, 500*time.Millisecond)

	if got := h.ConnectedCount(); got != 1 {
		t.Fatalf("expected dead client to be removed, ConnectedCount == %d", got)
	}
}

// drainSendChannel reads and discards any pending messages from a client's Send channel.
func drainSendChannel(t *testing.T, c *Client) {
	t.Helper()
	for {
		select {
		case <-c.Send:
		default:
			return
		}
	}
}

// readFromSend reads one message from the client's Send channel with a timeout.
func readFromSend(t *testing.T, c *Client, timeout time.Duration) []byte {
	t.Helper()
	select {
	case msg := <-c.Send:
		return msg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for message on Send channel")
		return nil
	}
}
