package sse

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/gin-gonic/gin"
)

// Event merepresentasikan payload data SSE
type Event struct {
	Type string      `json:"type"` // e.g. "stats", "new_email", "log"
	Data interface{} `json:"data"`
}

// Hub mengelola koneksi client SSE yang aktif dan broadcast pesan
type Hub struct {
	clients    map[chan Event]bool
	register   chan chan Event
	unregister chan chan Event
	broadcast  chan Event
	mutex      sync.RWMutex
}

var GlobalHub = NewHub()

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[chan Event]bool),
		register:   make(chan chan Event),
		unregister: make(chan chan Event),
		broadcast:  make(chan Event, 100),
	}
}

// Run menjalankan event loop pengelola client SSE
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			log.Printf("📡 Client SSE terhubung (Total aktif: %d)", len(h.clients))

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client)
			}
			h.mutex.Unlock()
			log.Printf("🔌 Client SSE terputus (Total aktif: %d)", len(h.clients))

		case event := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client <- event:
				default:
					// Jika buffer client penuh, lewati agar tidak memblokir goroutine lain
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// Broadcast mengirimkan event ke seluruh client yang sedang membuka dashboard
func (h *Hub) Broadcast(eventType string, data interface{}) {
	h.broadcast <- Event{
		Type: eventType,
		Data: data,
	}
}

// SSEHandler adalah Gin handler untuk endpoint SSE stream
func (h *Hub) SSEHandler(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	clientChan := make(chan Event, 10)
	h.register <- clientChan

	defer func() {
		h.unregister <- clientChan
	}()

	// Kirim pesan sambutan
	welcome, _ := json.Marshal(gin.H{"message": "Terhubung ke stream live SSE RaHaMail Server"})
	fmt.Fprintf(c.Writer, "event: connected\ndata: %s\n\n", welcome)
	c.Writer.Flush()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case event, ok := <-clientChan:
			if !ok {
				return false
			}
			dataBytes, err := json.Marshal(event.Data)
			if err != nil {
				return true
			}
			c.SSEvent(event.Type, string(dataBytes))
			return true
		}
	})
}
