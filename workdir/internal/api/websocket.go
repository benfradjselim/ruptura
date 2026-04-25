package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/benfradjselim/kairo-core/pkg/logger"
)

// newUpgrader returns a WebSocket upgrader that enforces the CORS origin allowlist.
// If allowedOrigins is empty, all origins are permitted (dev mode).
func newUpgrader(allowedOrigins []string) websocket.Upgrader {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if len(allowedOrigins) == 0 {
				return true // wildcard (dev mode)
			}
			origin := r.Header.Get("Origin")
			_, ok := allowed[origin]
			return ok
		},
	}
}

// Hub manages all WebSocket connections and broadcasts
type Hub struct {
	mu       sync.RWMutex
	clients  map[*wsClient]struct{}
	upgrader websocket.Upgrader
}

type wsClient struct {
	conn   *websocket.Conn
	send   chan []byte
	closed bool
	mu     sync.Mutex // protects closed flag and channel access
}

// safelySend sends msg to the client, returning false if the client is closed
func (c *wsClient) safelySend(msg []byte) (sent bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	select {
	case c.send <- msg:
		return true
	default:
		return false // slow client; drop
	}
}

// close marks the client closed and closes the channel exactly once
func (c *wsClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.send)
	}
}

// NewHub creates a new WebSocket hub with the given CORS origin allowlist.
// Pass nil or empty slice for wildcard (dev mode).
func NewHub(allowedOrigins []string) *Hub {
	return &Hub{
		clients:  make(map[*wsClient]struct{}),
		upgrader: newUpgrader(allowedOrigins),
	}
}

// Broadcast sends a message to all connected clients
func (hub *Hub) Broadcast(msg []byte) {
	hub.mu.RLock()
	clients := make([]*wsClient, 0, len(hub.clients))
	for c := range hub.clients {
		clients = append(clients, c)
	}
	hub.mu.RUnlock()

	for _, c := range clients {
		c.safelySend(msg)
	}
}

func (hub *Hub) register(c *wsClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.clients[c] = struct{}{}
}

func (hub *Hub) unregister(c *wsClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	delete(hub.clients, c)
	c.close()
}

// WebSocketHandler upgrades HTTP to WebSocket and streams live KPI/metric events
func (h *Handlers) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := h.hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Default.ErrorCtx(r.Context(), "ws upgrade error", "err", err)
		return
	}

	client := &wsClient{conn: conn, send: make(chan []byte, 256)}
	h.hub.register(client)

	// Writer goroutine: flushes the send channel to the WebSocket connection
	go func() {
		defer h.hub.unregister(client)
		pingTicker := time.NewTicker(30 * time.Second)
		defer pingTicker.Stop()

		for {
			select {
			case msg, ok := <-client.send:
				if !ok {
					_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-pingTicker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Reader: keep-alive pong handler; runs in handler goroutine
	conn.SetReadLimit(512)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
	// Ensure writer is signalled to exit
	client.close()
}
