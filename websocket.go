package supergin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketHandler defines the interface for WebSocket event handlers
type WebSocketHandler interface {
	OnConnect(conn *WebSocketConnection)
	OnDisconnect(conn *WebSocketConnection)
	OnMessage(conn *WebSocketConnection, messageType string, data interface{})
	OnError(conn *WebSocketConnection, err error)
}

// WebSocketConnection represents a WebSocket connection with metadata
type WebSocketConnection struct {
	ID       string
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *WebSocketHub
	User     interface{} // User context/session data
	Metadata map[string]interface{}
	mutex    sync.RWMutex
}

// WebSocketHub manages all WebSocket connections
type WebSocketHub struct {
	connections map[string]*WebSocketConnection
	register    chan *WebSocketConnection
	unregister  chan *WebSocketConnection
	broadcast   chan []byte
	handler     WebSocketHandler
	mutex       sync.RWMutex
}

// WebSocketMessage represents a structured WebSocket message
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	ID        string      `json:"id,omitempty"`
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize    int
	WriteBufferSize   int
	CheckOrigin       func(r *http.Request) bool
	EnableCompression bool
	HandshakeTimeout  time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	PingInterval      time.Duration
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(handler WebSocketHandler) *WebSocketHub {
	return &WebSocketHub{
		connections: make(map[string]*WebSocketConnection),
		register:    make(chan *WebSocketConnection),
		unregister:  make(chan *WebSocketConnection),
		broadcast:   make(chan []byte),
		handler:     handler,
	}
}

// Run starts the WebSocket hub
func (h *WebSocketHub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mutex.Lock()
			h.connections[conn.ID] = conn
			h.mutex.Unlock()

			if h.handler != nil {
				h.handler.OnConnect(conn)
			}

			log.Printf("WebSocket client connected: %s (total: %d)", conn.ID, len(h.connections))

		case conn := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.connections[conn.ID]; ok {
				delete(h.connections, conn.ID)
				close(conn.Send)
			}
			h.mutex.Unlock()

			if h.handler != nil {
				h.handler.OnDisconnect(conn)
			}

			log.Printf("WebSocket client disconnected: %s (total: %d)", conn.ID, len(h.connections))

		case message := <-h.broadcast:
			h.mutex.RLock()
			for _, conn := range h.connections {
				select {
				case conn.Send <- message:
				default:
					close(conn.Send)
					delete(h.connections, conn.ID)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHub) Broadcast(messageType string, data interface{}) error {
	message := WebSocketMessage{
		Type:      messageType,
		Data:      data,
		Timestamp: time.Now(),
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	h.broadcast <- msgBytes
	return nil
}

// SendToConnection sends a message to a specific connection
func (h *WebSocketHub) SendToConnection(connID string, messageType string, data interface{}) error {
	h.mutex.RLock()
	conn, exists := h.connections[connID]
	h.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("connection %s not found", connID)
	}

	return conn.Send(messageType, data)
}

// GetConnections returns all active connections
func (h *WebSocketHub) GetConnections() map[string]*WebSocketConnection {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	connections := make(map[string]*WebSocketConnection)
	for k, v := range h.connections {
		connections[k] = v
	}
	return connections
}

// Send sends a message through this connection
func (conn *WebSocketConnection) Send(messageType string, data interface{}) error {
	message := WebSocketMessage{
		Type:      messageType,
		Data:      data,
		Timestamp: time.Now(),
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	select {
	case conn.Send <- msgBytes:
		return nil
	default:
		return fmt.Errorf("connection send channel is full")
	}
}

// SetMetadata sets metadata for this connection
func (conn *WebSocketConnection) SetMetadata(key string, value interface{}) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	if conn.Metadata == nil {
		conn.Metadata = make(map[string]interface{})
	}
	conn.Metadata[key] = value
}

// GetMetadata gets metadata for this connection
func (conn *WebSocketConnection) GetMetadata(key string) (interface{}, bool) {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()

	if conn.Metadata == nil {
		return nil, false
	}
	value, exists := conn.Metadata[key]
	return value, exists
}

// Close closes the WebSocket connection
func (conn *WebSocketConnection) Close() {
	conn.Conn.Close()
}

// WebSocket route builder extension
func (rb *RouteBuilder) WebSocket(path string, handler WebSocketHandler) *RouteBuilder {
	hub := NewWebSocketHub(handler)

	// Start the hub in a goroutine
	go hub.Run()

	// Store hub in route metadata for access
	rb.WithMetadata("websocket_hub", hub)

	rb.GET(path).Handler(func(c *gin.Context) {
		handleWebSocketUpgrade(c, hub)
	})

	return rb
}

// Engine extension for WebSocket support
func (e *Engine) WebSocket(name, path string, handler WebSocketHandler) *WebSocketHub {
	hub := NewWebSocketHub(handler)
	go hub.Run()

	e.Named(name).
		GET(path).
		WithDescription(fmt.Sprintf("WebSocket endpoint: %s", name)).
		WithTags("websocket").
		WithMetadata("websocket_hub", hub).
		Handler(func(c *gin.Context) {
			handleWebSocketUpgrade(c, hub)
		})

	return hub
}

// handleWebSocketUpgrade handles the WebSocket upgrade
func handleWebSocketUpgrade(c *gin.Context, hub *WebSocketHub) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Generate unique connection ID
	connID := fmt.Sprintf("ws_%d", time.Now().UnixNano())

	wsConn := &WebSocketConnection{
		ID:       connID,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      hub,
		Metadata: make(map[string]interface{}),
	}

	// Register connection
	hub.register <- wsConn

	// Start goroutines for reading and writing
	go wsConn.writePump()
	go wsConn.readPump()
}

// readPump pumps messages from the WebSocket connection to the hub
func (conn *WebSocketConnection) readPump() {
	defer func() {
		conn.Hub.unregister <- conn
		conn.Conn.Close()
	}()

	conn.Conn.SetReadLimit(512)
	conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.Conn.SetPongHandler(func(string) error {
		conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
				if conn.Hub.handler != nil {
					conn.Hub.handler.OnError(conn, err)
				}
			}
			break
		}

		// Parse message
		var msg WebSocketMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			log.Printf("Failed to parse WebSocket message: %v", err)
			continue
		}

		// Handle message
		if conn.Hub.handler != nil {
			conn.Hub.handler.OnMessage(conn, msg.Type, msg.Data)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (conn *WebSocketConnection) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-conn.Send:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(conn.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-conn.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Default WebSocket handler implementation
type DefaultWebSocketHandler struct {
	OnConnectFunc    func(conn *WebSocketConnection)
	OnDisconnectFunc func(conn *WebSocketConnection)
	OnMessageFunc    func(conn *WebSocketConnection, messageType string, data interface{})
	OnErrorFunc      func(conn *WebSocketConnection, err error)
}

func (h *DefaultWebSocketHandler) OnConnect(conn *WebSocketConnection) {
	if h.OnConnectFunc != nil {
		h.OnConnectFunc(conn)
	}
}

func (h *DefaultWebSocketHandler) OnDisconnect(conn *WebSocketConnection) {
	if h.OnDisconnectFunc != nil {
		h.OnDisconnectFunc(conn)
	}
}

func (h *DefaultWebSocketHandler) OnMessage(conn *WebSocketConnection, messageType string, data interface{}) {
	if h.OnMessageFunc != nil {
		h.OnMessageFunc(conn, messageType, data)
	}
}

func (h *DefaultWebSocketHandler) OnError(conn *WebSocketConnection, err error) {
	if h.OnErrorFunc != nil {
		h.OnErrorFunc(conn, err)
	}
}
