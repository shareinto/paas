// internal/ws/hub.go
package ws

import (
	"context"
	"encoding/json"
	"sync"

	"nhooyr.io/websocket"
)

// Message is a server-to-client push message.
type Message struct {
	Type    string          `json:"type"`
	AppID   string          `json:"app_id"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Hub manages WebSocket connections and broadcasts messages to subscribers.
type Hub struct {
	mu    sync.RWMutex
	conns map[*client]struct{}
}

type client struct {
	conn   *websocket.Conn
	appIDs map[string]struct{}
	send   chan []byte
	done   chan struct{}
}

func NewHub() *Hub {
	return &Hub{conns: make(map[*client]struct{})}
}

// Broadcast sends a message to all clients subscribed to the given appID.
func (h *Hub) Broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.conns {
		if _, ok := c.appIDs[msg.AppID]; !ok {
			continue
		}
		select {
		case c.send <- data:
		default:
			// drop if client is slow
		}
	}
}

// Serve handles a new WebSocket connection. It blocks until the connection closes.
func (h *Hub) Serve(ctx context.Context, conn *websocket.Conn) {
	c := &client{
		conn:   conn,
		appIDs: make(map[string]struct{}),
		send:   make(chan []byte, 16),
		done:   make(chan struct{}),
	}
	h.mu.Lock()
	h.conns[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.conns, c)
		h.mu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	// Writer goroutine
	go func() {
		defer close(c.done)
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-c.send:
				if !ok {
					return
				}
				if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
					return
				}
			}
		}
	}()

	// Reader loop: process subscribe messages
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var msg struct {
			Type  string `json:"type"`
			AppID string `json:"app_id"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "subscribe":
			if msg.AppID != "" {
				c.appIDs[msg.AppID] = struct{}{}
			}
		case "unsubscribe":
			delete(c.appIDs, msg.AppID)
		}
	}
}
