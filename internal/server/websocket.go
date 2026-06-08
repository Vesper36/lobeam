package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

// WSHub manages WebSocket connections
type WSHub struct {
	clients    map[string]map[*WSClient]bool // room -> clients
	broadcast  chan *WSMessage
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
}

type WSClient struct {
	hub    *WSHub
	conn   *websocket.Conn
	room   string
	send   chan []byte
	userID int64
}

type WSMessage struct {
	Room    string      `json:"room"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[string]map[*WSClient]bool),
		broadcast:  make(chan *WSMessage, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.room] == nil {
				h.clients[client.room] = make(map[*WSClient]bool)
			}
			h.clients[client.room][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.room]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.clients, client.room)
				}
			}
			close(client.send)
			h.mu.Unlock()

		case msg := <-h.broadcast:
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			h.mu.Lock()
			if clients, ok := h.clients[msg.Room]; ok {
				for client := range clients {
					select {
					case client.send <- data:
					default:
						close(client.send)
						delete(clients, client)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *WSHub) BroadcastToRoom(room, msgType string, payload interface{}) {
	h.broadcast <- &WSMessage{
		Room:    room,
		Type:    msgType,
		Payload: payload,
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	room := r.URL.Query().Get("room")
	if room == "" {
		room = "global"
	}

	client := &WSClient{
		hub:  s.hub,
		conn: conn,
		room: room,
		send: make(chan []byte, 256),
	}

	s.hub.register <- client

	go client.writePump()
	client.readPump()
}

func (s *Server) handleP2PWebSocket(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	client := &WSClient{
		hub:  s.hub,
		conn: conn,
		room: "p2p:" + code,
		send: make(chan []byte, 256),
	}

	s.hub.register <- client

	go client.writePump()
	client.readPump()
}

func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		_, data, err := c.conn.Read(context.Background())
		if err != nil {
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		msg.Room = c.room

		// Relay to other clients in the room
		c.hub.broadcast <- &msg
	}
}

func (c *WSClient) writePump() {
	defer c.conn.Close(websocket.StatusNormalClosure, "")

	for data := range c.send {
		if err := c.conn.Write(context.Background(), websocket.MessageText, data); err != nil {
			break
		}
	}
}
