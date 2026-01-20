# SuperGin 🚀

An enhanced Gin framework for Go that provides Rails-like conventions, dependency injection, input/output validation, WebSocket support, and gRPC-HTTP bridging. SuperGin wraps the powerful Gin HTTP framework while adding modern web development features.

## ✨ Features

- 📛 **Named Routes** - Route registry with name-based URL generation
- 📦 **Input/Output Validation** - Automatic JSON binding and validation 
- 🏗️ **Dependency Injection** - Global DI container with singleton, request, and transient scopes
- 🛤️ **Rails-like REST Resources** - Convention over configuration for CRUD operations
- 📎 **Route Metadata & Annotations** - Rich route documentation and tagging
- 📄 **Automatic API Documentation** - Built-in endpoint for API introspection
- 🔌 **WebSocket Support** - Real-time bidirectional communication with connection management
- 🌉 **gRPC-HTTP Bridge** - Automatic conversion between gRPC and HTTP with protobuf support
- 🗜️ **Snappy Compression** - High-performance compression using Google's Snappy algorithm
- ⚡ **Programmable User Functions** - Custom middleware and function registry
- 🧪 **Testable Route Registry** - Easy testing and route verification
- 🔧 **Fluent API** - Chainable, readable route definitions

## 🚀 Quick Start

### Installation

```bash
go get github.com/supergin/supergin
```

### Basic Usage

```go
package main

import (
    "github.com/supergin/supergin"
    "github.com/gin-gonic/gin"
)

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=2"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=0,lte=130"`
}

type UserResponse struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func main() {
    // Create SuperGin engine
    app := supergin.New()

    // Traditional route with validation
    app.Named("create_user").
        POST("/users").
        WithIO(CreateUserRequest{}, UserResponse{}).
        WithDescription("Create a new user").
        WithTags("users", "create").
        Handler(func(c *gin.Context) {
            if input, exists := supergin.GetValidatedInput(c); exists {
                req := input.(*CreateUserRequest)
                // Handle validated request...
                response := User                response := UserResponse{
                    ID:    123,
                    Name:  req.Name,
                    Email: req.Email,
                    Age:   req.Age,
                }
                c.JSON(201, response)
            }
        })

    app.Run(":8080")
}
```

## 🏗️ Dependency Injection

SuperGin provides a powerful global DI container that eliminates context passing:

```go
// Setup DI
supergin.RegisterSingleton("database", func(config *DatabaseConfig) Database {
    return &PostgresDB{config: config}
}, "dbConfig")

supergin.RegisterRequest("userService", func(repo UserRepository) UserService {
    return &UserServiceImpl{repo: repo}
}, "userRepository")

// Use in handlers without context passing
func (uc *UserController) Create(c *gin.Context) {
    // Resolve service directly - no dependency drilling!
    userService := supergin.Resolve[UserService]("userService")
    
    user, err := userService.CreateUser(req)
    // ...
}
```

### DI Scopes

- **Singleton**: One instance for entire application
- **Request**: One instance per HTTP request (thread-safe)
- **Transient**: New instance every time

## 🛤️ Rails-like REST Resources

Generate full CRUD routes with convention over configuration:

```go
// Implement the CRUD interface
type UserController struct{}

func (uc *UserController) Create(c *gin.Context) { /* ... */ }
func (uc *UserController) Read(c *gin.Context)   { /* ... */ }
func (uc *UserController) Update(c *gin.Context) { /* ... */ }
func (uc *UserController) Delete(c *gin.Context) { /* ... */ }
func (uc *UserController) List(c *gin.Context)   { /* ... */ }
func (uc *UserController) Search(c *gin.Context) { /* ... */ }

// Generate REST routes
userRoutes := app.Resource("User", &UserController{}).
    WithModel(CreateUserRequest{}, UserResponse{}, UserSearchRequest{}).
    WithTags("api", "v1").
    WithMiddleware(authMiddleware).
    // Add custom routes
    Member("activate", "POST", "/activate", activateHandler).
    Collection("export", "GET", "/export", exportHandler).
    Build()
```

This generates:
- `GET /users` → List users
- `POST /users` → Create user  
- `GET /users/:id` → Get user
- `PUT /users/:id` → Update user
- `DELETE /users/:id` → Delete user
- `GET /users/search` → Search users
- `POST /users/:id/activate` → Custom member route
- `GET /users/export` → Custom collection route

## 🔌 WebSocket Support

Real-time bidirectional communication with connection management:

```go
// Implement WebSocket handler
type ChatHandler struct{}

func (h *ChatHandler) OnConnect(conn *supergin.WebSocketConnection) {
    log.Printf("Client connected: %s", conn.ID)
    conn.Send("welcome", map[string]interface{}{
        "message": "Welcome to the chat!",
    })
}

func (h *ChatHandler) OnMessage(conn *supergin.WebSocketConnection, messageType string, data interface{}) {
    // Handle different message types
    switch messageType {
    case "chat_message":
        // Broadcast to all connections
        conn.Hub.Broadcast("new_message", data)
    case "ping":
        conn.Send("pong", map[string]interface{}{"timestamp": time.Now()})
    }
}

// Register WebSocket endpoint
chatHub := app.WebSocket("chat", "/ws/chat", &ChatHandler{})

// Broadcast from anywhere in your application
chatHub.Broadcast("notification", map[string]interface{}{
    "type": "system",
    "message": "Server maintenance in 5 minutes",
})
```

### WebSocket Features

- **Connection Management**: Automatic connection tracking and cleanup
- **Message Broadcasting**: Send to all connections or specific ones
- **Connection Metadata**: Store user sessions and custom data
- **Event Handlers**: OnConnect, OnDisconnect, OnMessage, OnError
- **Hub Management**: Centralized connection management

## 🌉 gRPC-HTTP Bridge

Automatic bidirectional conversion between gRPC and HTTP:

```go
// Define your HTTP and gRPC types
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

type CreateUserGrpcRequest struct {
    Name  string `protobuf:"bytes,1,opt,name=name"`
    Email string `protobuf:"bytes,2,opt,name=email"`
    Age   int32  `protobuf:"varint,3,opt,name=age"`
}

// Implement custom conversion (optional)
func (req *CreateUserRequest) ToGrpc() (proto.Message, error) {
    return &CreateUserGrpcRequest{
        Name:  req.Name,
        Email: req.Email,
        Age:   int32(req.Age),
    }, nil
}

// Register gRPC service
bridge := app.GrpcBridge()
bridge.RegisterGrpcService("userService", "localhost:9090", "user.UserService")

// Create bidirectional bridge
app.BidirectionalGrpcHttp("user_create",
    "/api/users/grpc",           // HTTP endpoint
    "userService", "CreateUser", // gRPC service and method
    CreateUserRequest{}, UserResponse{},           // HTTP types
    &CreateUserGrpcRequest{}, &UserGrpcResponse{}) // gRPC types
```

This creates:
- **HTTP → gRPC**: `POST /api/users/grpc` forwards to gRPC service
- **gRPC → HTTP**: `POST /grpc/users/grpc` accepts gRPC and converts to HTTP

### gRPC Bridge Features

- **Automatic Conversion**: JSON ↔ Protobuf conversion
- **Custom Converters**: Implement `GrpcConverter` interface for custom logic
- **Bidirectional**: Both HTTP→gRPC and gRPC→HTTP
- **Type Safety**: Compile-time type checking
- **Metadata Handling**: HTTP headers ↔ gRPC metadata

## 🗜️ Snappy Compression

SuperGin includes high-performance compression using Google's Snappy algorithm:

```go
// Enable Snappy compression for all responses
app.Use(supergin.SnappyCompressionMiddleware(supergin.CompressionConfig{
    Type:    supergin.CompressionSnappy,
    MinSize: 1024, // Only compress responses larger than 1KB
}))

// Or use gzip compression
app.Use(supergin.SnappyCompressionMiddleware(supergin.CompressionConfig{
    Type:  supergin.CompressionGzip,
    Level: 6, // Compression level 1-9
}))

// Both request decompression and response compression
app.Use(supergin.CompressionMiddleware())
```

### Compression Features

- **Multiple Algorithms**: Snappy (fast) or Gzip (smaller)
- **Automatic Detection**: Only compresses when beneficial
- **Configurable**: Set minimum size, excluded paths, and file types
- **Request Decompression**: Automatically decompress incoming requests
- **Headers**: Adds `Content-Encoding` and size headers

## ⚡ Programmable User Functions

Register and apply custom middleware functions dynamically:

```go
// Register a custom logging function
_, logHandler := supergin.LoggingFunction("api_logger")
app.RegisterUserFunction("api_logger", "API request logger", logHandler)

// Register a custom authentication function
_, authHandler := supergin.AuthenticationFunction("jwt_auth", func(c *gin.Context) bool {
    token := c.GetHeader("Authorization")
    return validateJWT(token) // Your validation logic
})
app.RegisterUserFunction("jwt_auth", "JWT authentication", authHandler)

// Apply user functions to routes
app.Named("protected_route").
    GET("/api/protected").
    ApplyUserFunction("jwt_auth").
    ApplyUserFunction("api_logger").
    Handler(protectedHandler)

// Chain multiple user functions
app.Named("complex_route").
    GET("/api/complex").
    ChainUserFunctions("auth", "logger", "timer").
    Handler(complexHandler)

// List all registered user functions
functions := app.ListUserFunctions()
```

### Built-in User Function Builders

- **LoggingFunction**: Custom request logging
- **TimingFunction**: Request timing and performance monitoring
- **AuthenticationFunction**: Custom authentication with validator
- **TransformFunction**: Request/response transformation

## 📦 Input/Output Validation

Automatic validation using struct tags:

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=2"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=0,lte=130"`
}

app.Named("create_user").
    POST("/users").
    WithInput(CreateUserRequest{}).  // Automatic validation
    Handler(func(c *gin.Context) {
        // Input is pre-validated and available
        if input, exists := supergin.GetValidatedInput(c); exists {
            req := input.(*CreateUserRequest)
            // req is guaranteed to be valid
        }
    })
```

## 📛 Named Routes & URL Generation

```go
// Define named routes
app.Named("show_user").GET("/users/:id").Handler(handler)

// Generate URLs
url, _ := app.URLFor("show_user", "id", "123")
// Returns: "/users/123"

// Access route metadata
route, exists := app.GetRoute("show_user")
if exists {
    fmt.Printf("Route: %s %s", route.Method, route.Path)
}
```

## 📄 API Documentation

Built-in documentation endpoint:

```bash
# View all routes, DI services, and WebSocket endpoints
curl http://localhost:8080/docs

# Returns:
{
  "routes": {
    "create_user": {
      "method": "POST",
      "path": "/users",
      "description": "Create a new user",
      "tags": ["users", "create"]
    }
  },
  "di_services": {
    "userService": {
      "type": "UserService",
      "scope": "request",
      "dependencies": ["userRepository"]
    }
  }
}
```

## 🧪 Testing

SuperGin makes testing easy with route registry and DI:

```go
func TestUserRoutes(t *testing.T) {
    app := supergin.New()
    
    // Setup test routes
    setupRoutes(app)
    
    // Verify routes exist
    assert.True(t, app.HasRoute("create_user"))
    
    // Test route metadata
    route, _ := app.GetRoute("create_user")
    assert.Equal(t, "POST", route.Method)
    assert.Equal(t, "/users", route.Path)
    
    // Test DI services
    services := app.DI().ListServices()
    assert.Contains(t, services, "userService")
    
    // Test WebSocket functionality
    hub := app.GetWebSocketHub("chat")
    assert.NotNil(t, hub)
}
```

## 🔧 Configuration

```go
app := supergin.New(supergin.Config{
    EnableDocs:     true,          // Enable /docs endpoint
    ValidateInput:  true,          // Enable input validation
    ValidateOutput: false,         // Enable output validation
    DocsPath:       "/api/docs",   // Custom docs path
})
```

## 📂 Project Structure

```
github.com/supergin/supergin/
├── go.mod                 # Go module with dependencies
├── supergin.go           # Main engine and core types
├── route_builder.go      # Fluent route building API
├── di.go                 # Dependency injection system
├── resource.go           # Rails-like REST resources
├── websocket.go          # WebSocket connection management
├── grpc_bridge.go        # gRPC-HTTP bidirectional bridge
├── errors.go             # Error types and handling
├── examples/
│   ├── basic/main.go     # Basic HTTP API example
│   └── advanced/main.go  # Advanced example with WebSocket + gRPC
├── Makefile              # Build and development tasks
└── README.md             # This file
```

## 🛠️ Development

```bash
# Run basic example
make run-example

# Run advanced example with WebSocket and gRPC
cd examples/advanced && go run main.go

# Run tests
make test

# Format and lint
make fmt vet

# Build for release
make build-release
```

## 📚 Examples

### Basic Example
See [`examples/basic/main.go`](examples/basic/main.go) for HTTP API with DI.

### Advanced Example  
See [`examples/advanced/main.go`](examples/advanced/main.go) for a complete application featuring:

- ✅ HTTP API with dependency injection
- ✅ Real-time WebSocket chat with connection management
- ✅ gRPC-HTTP bidirectional bridge
- ✅ Custom type conversion
- ✅ Interactive chat demo webpage
- ✅ REST resource generation  
- ✅ Input/output validation
- ✅ Route introspection

```bash
# Run the advanced example
cd examples/advanced
go run main.go

# Visit the chat demo
open http://localhost:8080/chat
```

## 🚀 Quick Start with All Features

```go
package main

import "github.com/supergin/supergin"

func main() {
    app := supergin.New()
    
    // Dependency Injection
    supergin.RegisterSingleton("service", func() MyService {
        return &MyServiceImpl{}
    })
    
    // REST Resources  
    app.Resource("User", &UserController{}).
        WithModel(CreateUserRequest{}, UserResponse{}, UserSearchRequest{}).
        Build()
    
    // WebSocket
    app.WebSocket("chat", "/ws/chat", &ChatHandler{})
    
    // gRPC Bridge
    app.BidirectionalGrpcHttp("grpc_endpoint",
        "/api/grpc", "service", "method",
        HTTPInput{}, HTTPOutput{},
        &GrpcInput{}, &GrpcOutput{})
    
    app.Run(":8080")
}
```

## 🤝 Contributing

Contributions welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is licensed under the MIT License.

## 🙏 Acknowledgments

- Built on top of [Gin](https://github.com/gin-gonic/gin)
- WebSocket support via [Gorilla WebSocket](https://github.com/gorilla/websocket)  
- gRPC integration with [gRPC-Go](https://google.golang.org/grpc)
- Validation by [go-playground/validator](https://github.com/go-playground/validator)
- Compression powered by [Google Snappy](https://github.com/google/snappy)

---

**SuperGin** - The complete Go web framework for modern applications! 🚀Response{
                    ID:    123,
                    Name:  req.Name,
                    Email: req.Email,
                    Age:   req.Age,
                }
                c.JSON(201, response)
            }
        })

    app.Run(":8080")
}
```

## 🏗️ Dependency Injection

SuperGin provides a powerful global DI container that eliminates context passing:

```go
// Setup DI
supergin.RegisterSingleton("database", func(config *DatabaseConfig) Database {
    return &PostgresDB{config: config}
}, "dbConfig")

supergin.RegisterRequest("userService", func(repo UserRepository) UserService {
    return &UserServiceImpl{repo: repo}
}, "userRepository")

// Use in handlers without context passing
func (uc *UserController) Create(c *gin.Context) {
    // Resolve service directly - no dependency drilling!
    userService := supergin.Resolve[UserService]("userService")
    
    user, err := userService.CreateUser(req)
    // ...
}
```

### DI Scopes

- **Singleton**: One instance for entire application
- **Request**: One instance per HTTP request (thread-safe)
- **Transient**: New instance every time

## 🛤️ Rails-like REST Resources

Generate full CRUD routes with convention over configuration:

```go
// Implement the CRUD interface
type UserController struct{}

func (uc *UserController) Create(c *gin.Context) { /* ... */ }
func (uc *UserController) Read(c *gin.Context)   { /* ... */ }
func (uc *UserController) Update(c *gin.Context) { /* ... */ }
func (uc *UserController) Delete(c *gin.Context) { /* ... */ }
func (uc *UserController) List(c *gin.Context)   { /* ... */ }
func (uc *UserController) Search(c *gin.Context) { /* ... */ }

// Generate REST routes
userRoutes := app.Resource("User", &UserController{}).
    WithModel(CreateUserRequest{}, UserResponse{}, UserSearchRequest{}).
    WithTags("api", "v1").
    WithMiddleware(authMiddleware).
    // Add custom routes
    Member("activate", "POST", "/activate", activateHandler).
    Collection("export", "GET", "/export", exportHandler).
    Build()
```

This generates:
- `GET /users` → List users
- `POST /users` → Create user  
- `GET /users/:id` → Get user
- `PUT /users/:id` → Update user
- `DELETE /users/:id` → Delete user
- `GET /users/search` → Search users
- `POST /users/:id/activate` → Custom member route
- `GET /users/export` → Custom collection route

## 📦 Input/Output Validation

Automatic validation using struct tags:

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=2"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=0,lte=130"`
}

app.Named("create_user").
    POST("/users").
    WithInput(CreateUserRequest{}).  // Automatic validation
    Handler(func(c *gin.Context) {
        // Input is pre-validated and available
        if input, exists := supergin.GetValidatedInput(c); exists {
            req := input.(*CreateUserRequest)
            // req is guaranteed to be valid
        }
    })
```

## 📛 Named Routes & URL Generation

```go
// Define named routes
app.Named("show_user").GET("/users/:id").Handler(handler)

// Generate URLs
url, _ := app.URLFor("show_user", "id", "123")
// Returns: "/users/123"

// Access route metadata
route, exists := app.GetRoute("show_user")
if exists {
    fmt.Printf("Route: %s %s", route.Method, route.Path)
}
```

## 📄 API Documentation

Built-in documentation endpoint:

```bash
# View all routes and DI services
curl http://localhost:8080/docs

# Returns:
{
  "routes": {
    "create_user": {
      "method": "POST",
      "path": "/users",
      "description": "Create a new user",
      "tags": ["users", "create"]
    }
  },
  "di_services": {
    "userService": {
      "type": "UserService",
      "scope": "request",
      "dependencies": ["userRepository"]
    }
  }
}
```

## 🧪 Testing

SuperGin makes testing easy with route registry and DI:

```go
func TestUserRoutes(t *testing.T) {
    app := supergin.New()
    
    // Setup test routes
    setupRoutes(app)
    
    // Verify routes exist
    assert.True(t, app.HasRoute("create_user"))
    
    // Test route metadata
    route, _ := app.GetRoute("create_user")
    assert.Equal(t, "POST", route.Method)
    assert.Equal(t, "/users", route.Path)
    
    // Test DI services
    services := app.DI().ListServices()
    assert.Contains(t, services, "userService")
}
```

## 🔧 Configuration

```go
app := supergin.New(supergin.Config{
    EnableDocs:     true,          // Enable /docs endpoint
    ValidateInput:  true,          // Enable input validation
    ValidateOutput: false,         // Enable output validation
    DocsPath:       "/api/docs",   // Custom docs path
})
```

## 📂 Project Structure

```
github.com/supergin/supergin/
├── go.mod                 # Go module definition
├── supergin.go           # Main engine and core types
├── route_builder.go      # Fluent route building API
├── di.go                 # Dependency injection system
├── resource.go           # Rails-like REST resources
├── errors.go             # Error types and handling
├── examples/
│   └── basic/
│       └── main.go       # Complete working example
├── Makefile              # Build and development tasks
└── README.md             # This file
```

## 🛠️ Development

```bash
# Run basic example
make run-example

# Run tests
make test

# Format and lint
make fmt vet

# Build for release
make build-release

# Create new project
make init-project
```

## 📚 Examples

See the [`examples/basic/main.go`](examples/basic/main.go) for a complete working application demonstrating:

- ✅ Dependency injection setup
- ✅ REST resource generation  
- ✅ Custom routes and middleware
- ✅ Input/output validation
- ✅ Route introspection
- ✅ Error handling

## 🤝 Contributing

Contributions welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Acknowledgments

- Built on top of the excellent [Gin](https://github.com/gin-gonic/gin) framework
- Inspired by Rails conventions and Django patterns
- Validation powered by [go-playground/validator](https://github.com/go-playground/validator)

---

**SuperGin** - Making Go web development more productive! 🚀