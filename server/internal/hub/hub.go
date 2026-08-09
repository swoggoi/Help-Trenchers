package hub

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // десктоп локально/VPN — разрешаем все
}

// Hub рассылает события всем подключённым десктоп-клиентам.
type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func New() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("hub: upgrade: %v", err)
		return
	}
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
	log.Printf("hub: client connected (%d)", len(h.clients))

	// читаем (ping/pong), чтобы детектить отвал
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// Broadcast шлёт JSON всем клиентам.
func (h *Hub) Broadcast(v any) {
	msg, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

// EnrichFunc обогащает событие перед отправкой (например, флаг autoOpen).
func (h *Hub) EnrichAndBroadcast(payload map[string]any) {
	h.Broadcast(payload)
}
