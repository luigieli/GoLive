package hub

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxSendChannel = 128
)

type WSClient struct {
	hub  *WSHub
	conn *websocket.Conn
	send chan []byte
}

func NewWSClient(hub *WSHub, conn *websocket.Conn) *WSClient {
	return &WSClient{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, maxSendChannel),
	}
}

func (c *WSClient) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if c.conn != nil {
			_ = c.conn.Close()
		}
	}()

	for {
		select {
		case message, ok := <-c.send:
			if c.conn == nil {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			if c.conn == nil {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *WSClient) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		if c.conn != nil {
			_ = c.conn.Close()
		}
	}()
	if c.conn != nil {
		c.conn.SetReadLimit(512)
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.conn.SetPongHandler(func(string) error {
			_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})
		for {
			if _, _, err := c.conn.ReadMessage(); err != nil {
				break
			}
		}
	}
}

type WSHub struct {
	clients    map[*WSClient]bool
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan []byte, 4096),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Drop packet if client queue is saturated to prevent latency accumulation
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *WSHub) Register(c *WSClient) {
	h.register <- c
}

func (h *WSHub) Unregister(c *WSClient) {
	h.unregister <- c
}

func (h *WSHub) Broadcast(data []byte) {
	if len(data) == 0 {
		return
	}
	chunk := make([]byte, len(data))
	copy(chunk, data)

	select {
	case h.broadcast <- chunk:
	default:
		// Broadcast channel saturated; drop to avoid blocking
	}
}

func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
