package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ivikasavnish/supergin"
)

// HTTP Models
type CreateUserRequest struct {
	Name  string `json:"name" validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=130"`
}

type UserResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatMessage struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "message", "join", "leave"
}

// NOTE: gRPC bridge functionality requires actual .proto files and generated code.
// This example focuses on WebSocket and HTTP API features.
// For a complete gRPC bridge example, see the SuperGin documentation.

// Services (using DI)
type UserService interface {
	CreateUser(req *CreateUserRequest) (*UserResponse, error)
	GetUser(id int) (*UserResponse, error)
}

type UserServiceImpl struct{}

func (s *UserServiceImpl) CreateUser(req *CreateUserRequest) (*UserResponse, error) {
	return &UserResponse{
		ID:        123,
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		CreatedAt: time.Now(),
	}, nil
}

func (s *UserServiceImpl) GetUser(id int) (*UserResponse, error) {
	return &UserResponse{
		ID:        id,
		Name:      "John Doe",
		Email:     "john@example.com",
		Age:       30,
		CreatedAt: time.Now(),
	}, nil
}

// Chat service for WebSocket demo
type ChatService interface {
	BroadcastMessage(msg *ChatMessage)
	GetChatHistory() []*ChatMessage
	AddMessage(msg *ChatMessage)
}

type ChatServiceImpl struct {
	messages []*ChatMessage
	hub      *supergin.WebSocketHub
}

func (s *ChatServiceImpl) BroadcastMessage(msg *ChatMessage) {
	s.AddMessage(msg)
	if s.hub != nil {
		s.hub.Broadcast("chat_message", msg)
	}
}

func (s *ChatServiceImpl) GetChatHistory() []*ChatMessage {
	return s.messages
}

func (s *ChatServiceImpl) AddMessage(msg *ChatMessage) {
	s.messages = append(s.messages, msg)
	// Keep only last 100 messages
	if len(s.messages) > 100 {
		s.messages = s.messages[len(s.messages)-100:]
	}
}

// WebSocket handler for chat
type ChatWebSocketHandler struct {
	chatService ChatService
}

func (h *ChatWebSocketHandler) OnConnect(conn *supergin.WebSocketConnection) {
	log.Printf("WebSocket client connected: %s", conn.ID)

	// Send chat history to new connection
	history := h.chatService.GetChatHistory()
	conn.Send("chat_history", map[string]interface{}{
		"messages": history,
		"count":    len(history),
	})

	// Notify others about new user
	h.chatService.BroadcastMessage(&ChatMessage{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		UserID:    conn.ID,
		Username:  "Anonymous",
		Message:   "joined the chat",
		Type:      "join",
		Timestamp: time.Now(),
	})
}

func (h *ChatWebSocketHandler) OnDisconnect(conn *supergin.WebSocketConnection) {
	log.Printf("WebSocket client disconnected: %s", conn.ID)

	// Get username from metadata
	username := "Anonymous"
	if user, exists := conn.GetMetadata("username"); exists {
		username = user.(string)
	}

	// Notify others about user leaving
	h.chatService.BroadcastMessage(&ChatMessage{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		UserID:    conn.ID,
		Username:  username,
		Message:   "left the chat",
		Type:      "leave",
		Timestamp: time.Now(),
	})
}

func (h *ChatWebSocketHandler) OnMessage(conn *supergin.WebSocketConnection, messageType string, data interface{}) {
	switch messageType {
	case "set_username":
		if dataMap, ok := data.(map[string]interface{}); ok {
			if username, ok := dataMap["username"].(string); ok {
				conn.SetMetadata("username", username)
				conn.Send("username_set", map[string]interface{}{
					"username": username,
					"status":   "success",
				})
			}
		}

	case "chat_message":
		if dataMap, ok := data.(map[string]interface{}); ok {
			username := "Anonymous"
			if user, exists := conn.GetMetadata("username"); exists {
				username = user.(string)
			}

			message := &ChatMessage{
				ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				UserID:    conn.ID,
				Username:  username,
				Message:   dataMap["message"].(string),
				Type:      "message",
				Timestamp: time.Now(),
			}

			h.chatService.BroadcastMessage(message)
		}

	case "ping":
		conn.Send("pong", map[string]interface{}{
			"timestamp": time.Now(),
		})
	}
}

func (h *ChatWebSocketHandler) OnError(conn *supergin.WebSocketConnection, err error) {
	log.Printf("WebSocket error for connection %s: %v", conn.ID, err)
}

// Controllers
type UserController struct{}

func (uc *UserController) CreateUser(c *gin.Context) {
	userService := supergin.Resolve[UserService]("userService")

	if input, exists := supergin.GetValidatedInput(c); exists {
		req := input.(*CreateUserRequest)
		user, err := userService.CreateUser(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, user)
	}
}

func (uc *UserController) GetUser(c *gin.Context) {
	userService := supergin.Resolve[UserService]("userService")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	user, err := userService.GetUser(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func main() {
	// Setup Dependency Injection
	setupDI()

	// Create SuperGin engine
	app := supergin.New(supergin.Config{
		EnableDocs:     true,
		ValidateInput:  true,
		ValidateOutput: false,
		DocsPath:       "/api/docs",
	})

	// Setup WebSocket chat
	setupWebSocket(app)

	// Setup HTTP routes
	setupRoutes(app)

	// Start server
	fmt.Println("🚀 SuperGin Advanced Server starting on :8080")
	fmt.Println("")
	fmt.Println("📚 API Documentation: http://localhost:8080/api/docs")
	fmt.Println("👥 Users API: http://localhost:8080/api/users")
	fmt.Println("💬 WebSocket Chat: ws://localhost:8080/ws/chat")
	fmt.Println("🌐 Chat Demo: http://localhost:8080/chat")
	fmt.Println("")

	app.Run(":8080")
}

func setupDI() {
	// Register services
	supergin.RegisterSingleton("userService", func() UserService {
		return &UserServiceImpl{}
	})

	supergin.RegisterSingleton("chatService", func() ChatService {
		return &ChatServiceImpl{
			messages: make([]*ChatMessage, 0),
		}
	})

	fmt.Println("✅ Dependency injection configured")
}

// setupGrpcBridge is disabled in this example as it requires actual .proto files
// For gRPC bridge functionality, generate proper protobuf types using protoc
// and refer to the SuperGin documentation for complete setup instructions.

func setupWebSocket(app *supergin.Engine) {
	chatService := supergin.Resolve[ChatService]("chatService")

	// Create WebSocket handler
	chatHandler := &ChatWebSocketHandler{
		chatService: chatService,
	}

	// Register WebSocket endpoint
	chatHub := app.WebSocket("chat_ws", "/ws/chat", chatHandler)

	// Store hub reference in chat service for broadcasting
	if chatServiceImpl, ok := chatService.(*ChatServiceImpl); ok {
		chatServiceImpl.hub = chatHub
	}

	// Add HTTP endpoint for chat demo page
	app.Named("chat_demo").
		GET("/chat").
		WithDescription("Chat demo page").
		WithTags("demo", "chat").
		Handler(func(c *gin.Context) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(chatHTML))
		})

	// Add REST endpoint to get chat history
	app.Named("chat_history").
		GET("/api/chat/history").
		WithDescription("Get chat message history").
		WithTags("chat", "api").
		Handler(func(c *gin.Context) {
			chatService := supergin.Resolve[ChatService]("chatService")
			history := chatService.GetChatHistory()
			c.JSON(http.StatusOK, gin.H{
				"messages": history,
				"count":    len(history),
			})
		})

	fmt.Println("✅ WebSocket chat configured")
}

func setupRoutes(app *supergin.Engine) {
	// Traditional HTTP routes
	userController := &UserController{}

	// User management routes
	app.Named("create_user_http").
		POST("/api/users").
		WithIO(CreateUserRequest{}, UserResponse{}).
		WithDescription("Create a new user via HTTP").
		WithTags("users", "http").
		Handler(userController.CreateUser)

	app.Named("get_user_http").
		GET("/api/users/:id").
		WithOutput(UserResponse{}).
		WithDescription("Get user by ID via HTTP").
		WithTags("users", "http").
		Handler(userController.GetUser)

	// Health check
	app.Named("health").
		GET("/health").
		WithDescription("Health check endpoint").
		WithTags("health").
		Handler(func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"timestamp": time.Now(),
				"version":   "2.0.0",
				"features":  []string{"http", "websocket", "di"},
				"websocket": "enabled",
			})
		})

	// Demo endpoints
	app.Named("features_demo").
		GET("/demo/features").
		WithDescription("Demonstrate SuperGin features").
		WithTags("demo").
		Handler(func(c *gin.Context) {
			routes := app.GetRoutes()
			services := app.DI().ListServices()

			c.JSON(http.StatusOK, gin.H{
				"message": "SuperGin Advanced Features Demo",
				"features": gin.H{
					"total_routes":      len(routes),
					"di_services":       len(services),
					"websocket_enabled": true,
					"input_validation":  true,
					"named_routes":      true,
				},
				"endpoints": gin.H{
					"api_docs":       "/api/docs",
					"chat_demo":      "/chat",
					"chat_websocket": "ws://localhost:8080/ws/chat",
					"users_api":      "/api/users",
					"health":         "/health",
				},
			})
		})

	fmt.Println("✅ HTTP routes configured")
}

// Simple HTML for WebSocket chat demo
const chatHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>SuperGin WebSocket Chat Demo</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        #messages { border: 1px solid #ccc; height: 400px; overflow-y: scroll; padding: 10px; margin: 10px 0; }
        .message { margin: 5px 0; padding: 5px; }
        .join { color: green; font-style: italic; }
        .leave { color: red; font-style: italic; }
        .message-text { background: #f0f0f0; border-radius: 5px; padding: 5px; }
        input, button { padding: 10px; margin: 5px; }
        #messageInput { width: 60%; }
        button { background: #007cba; color: white; border: none; cursor: pointer; }
        button:hover { background: #005a87; }
        .status { color: #666; font-size: 0.9em; }
    </style>
</head>
<body>
    <h1>🚀 SuperGin WebSocket Chat Demo</h1>
    
    <div>
        <input type="text" id="usernameInput" placeholder="Enter your username" />
        <button onclick="setUsername()">Set Username</button>
    </div>
    
    <div id="messages"></div>
    
    <div>
        <input type="text" id="messageInput" placeholder="Type a message..." onkeypress="handleKeyPress(event)" />
        <button onclick="sendMessage()">Send</button>
        <button onclick="disconnect()">Disconnect</button>
    </div>
    
    <div class="status">
        Status: <span id="status">Connecting...</span>
    </div>

    <script>
        let ws;
        let connected = false;
        
        function connect() {
            ws = new WebSocket('ws://localhost:8080/ws/chat');
            
            ws.onopen = function() {
                connected = true;
                document.getElementById('status').textContent = 'Connected';
                addMessage('System', 'Connected to chat server', 'join');
            };
            
            ws.onmessage = function(event) {
                const data = JSON.parse(event.data);
                handleMessage(data);
            };
            
            ws.onclose = function() {
                connected = false;
                document.getElementById('status').textContent = 'Disconnected';
                addMessage('System', 'Disconnected from chat server', 'leave');
            };
            
            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
                document.getElementById('status').textContent = 'Error';
            };
        }
        
        function handleMessage(data) {
            switch(data.type) {
                case 'chat_message':
                    addMessage(data.data.username, data.data.message, data.data.type);
                    break;
                case 'chat_history':
                    data.data.messages.forEach(msg => {
                        addMessage(msg.username, msg.message, msg.type);
                    });
                    break;
                case 'username_set':
                    addMessage('System', 'Username set to: ' + data.data.username, 'join');
                    break;
                case 'pong':
                    console.log('Pong received:', data.data.timestamp);
                    break;
            }
        }
        
        function addMessage(username, message, type = 'message') {
            const messages = document.getElementById('messages');
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message ' + type;
            
            if (type === 'message') {
                messageDiv.innerHTML = '<strong>' + username + ':</strong> <span class="message-text">' + message + '</span>';
            } else {
                messageDiv.innerHTML = '<em>' + username + ' ' + message + '</em>';
            }
            
            messages.appendChild(messageDiv);
            messages.scrollTop = messages.scrollHeight;
        }
        
        function setUsername() {
            const username = document.getElementById('usernameInput').value.trim();
            if (username && connected) {
                ws.send(JSON.stringify({
                    type: 'set_username',
                    data: { username: username }
                }));
            }
        }
        
        function sendMessage() {
            const input = document.getElementById('messageInput');
            const message = input.value.trim();
            
            if (message && connected) {
                ws.send(JSON.stringify({
                    type: 'chat_message',
                    data: { message: message }
                }));
                input.value = '';
            }
        }
        
        function handleKeyPress(event) {
            if (event.key === 'Enter') {
                sendMessage();
            }
        }
        
        function disconnect() {
            if (ws) {
                ws.close();
            }
        }
        
        // Auto-connect on page load
        connect();
        
        // Send ping every 30 seconds to keep connection alive
        setInterval(() => {
            if (connected) {
                ws.send(JSON.stringify({ type: 'ping', data: {} }));
            }
        }, 30000);
    </script>
</body>
</html>
`
