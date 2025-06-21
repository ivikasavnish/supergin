# SuperGin 🚀

An enhanced Gin framework for Go that provides Rails-like conventions, dependency injection, input/output validation, and more. SuperGin wraps the powerful Gin HTTP framework while adding structure and productivity features.

## ✨ Features

- 📛 **Named Routes** - Route registry with name-based URL generation
- 📦 **Input/Output Validation** - Automatic JSON binding and validation 
- 🏗️ **Dependency Injection** - Global DI container with singleton, request, and transient scopes
- 🛤️ **Rails-like REST Resources** - Convention over configuration for CRUD operations
- 📎 **Route Metadata & Annotations** - Rich route documentation and tagging
- 📄 **Automatic API Documentation** - Built-in endpoint for API introspection
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
                response := UserResponse{
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