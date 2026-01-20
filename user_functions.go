package supergin

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// UserFunction represents a user-defined programmable function
type UserFunction struct {
	Name        string
	Description string
	Handler     gin.HandlerFunc
	Metadata    map[string]interface{}
}

// UserFunctionRegistry manages user-defined functions
type UserFunctionRegistry struct {
	functions map[string]*UserFunction
	mutex     sync.RWMutex
}

// NewUserFunctionRegistry creates a new user function registry
func NewUserFunctionRegistry() *UserFunctionRegistry {
	return &UserFunctionRegistry{
		functions: make(map[string]*UserFunction),
	}
}

// RegisterFunction registers a new user-defined function
func (r *UserFunctionRegistry) RegisterFunction(name, description string, handler gin.HandlerFunc) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.functions[name]; exists {
		return fmt.Errorf("function %s already registered", name)
	}

	r.functions[name] = &UserFunction{
		Name:        name,
		Description: description,
		Handler:     handler,
		Metadata:    make(map[string]interface{}),
	}

	return nil
}

// GetFunction retrieves a registered user function
func (r *UserFunctionRegistry) GetFunction(name string) (*UserFunction, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	fn, exists := r.functions[name]
	return fn, exists
}

// ListFunctions returns all registered functions
func (r *UserFunctionRegistry) ListFunctions() map[string]*UserFunction {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	functions := make(map[string]*UserFunction)
	for k, v := range r.functions {
		functions[k] = v
	}
	return functions
}

// UnregisterFunction removes a user-defined function
func (r *UserFunctionRegistry) UnregisterFunction(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.functions[name]; !exists {
		return fmt.Errorf("function %s not found", name)
	}

	delete(r.functions, name)
	return nil
}

// Add user function registry to Engine
func (e *Engine) RegisterUserFunction(name, description string, handler gin.HandlerFunc) error {
	// Note: e.userFunctions is initialized in New(), so this should never be nil
	// But we check anyway for safety
	if e.userFunctions == nil {
		return fmt.Errorf("user function registry not initialized")
	}
	return e.userFunctions.RegisterFunction(name, description, handler)
}

// GetUserFunction retrieves a user-defined function from the engine
func (e *Engine) GetUserFunction(name string) (*UserFunction, bool) {
	if e.userFunctions == nil {
		return nil, false
	}
	return e.userFunctions.GetFunction(name)
}

// ListUserFunctions returns all registered user functions
func (e *Engine) ListUserFunctions() map[string]*UserFunction {
	if e.userFunctions == nil {
		return make(map[string]*UserFunction)
	}
	return e.userFunctions.ListFunctions()
}

// ApplyUserFunction applies a registered user function to a route
// If the function is not found, it logs a warning and continues
func (rb *RouteBuilder) ApplyUserFunction(name string) *RouteBuilder {
	if rb.engine.userFunctions == nil {
		fmt.Printf("Warning: user function registry not initialized, cannot apply '%s'\n", name)
		return rb
	}

	fn, exists := rb.engine.userFunctions.GetFunction(name)
	if !exists {
		fmt.Printf("Warning: user function '%s' not found, skipping\n", name)
		return rb
	}

	return rb.WithMiddleware(fn.Handler)
}

// ChainUserFunctions applies multiple user functions in order
func (rb *RouteBuilder) ChainUserFunctions(names ...string) *RouteBuilder {
	for _, name := range names {
		rb = rb.ApplyUserFunction(name)
	}
	return rb
}

// Predefined user function builders

// LoggingFunction creates a logging user function
func LoggingFunction(name string) (*UserFunction, gin.HandlerFunc) {
	handler := func(c *gin.Context) {
		fmt.Printf("[%s] %s %s\n", name, c.Request.Method, c.Request.URL.Path)
		c.Next()
	}
	
	return &UserFunction{
		Name:        name,
		Description: "Custom logging function",
		Handler:     handler,
		Metadata:    map[string]interface{}{"type": "logging"},
	}, handler
}

// TimingFunction creates a timing user function
func TimingFunction(name string) (*UserFunction, gin.HandlerFunc) {
	handler := func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		fmt.Printf("[%s] Request took %v\n", name, duration)
	}
	
	return &UserFunction{
		Name:        name,
		Description: "Request timing function",
		Handler:     handler,
		Metadata:    map[string]interface{}{"type": "timing"},
	}, handler
}

// AuthenticationFunction creates an authentication user function
func AuthenticationFunction(name string, validator func(*gin.Context) bool) (*UserFunction, gin.HandlerFunc) {
	handler := func(c *gin.Context) {
		if !validator(c) {
			c.AbortWithStatusJSON(401, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
	
	return &UserFunction{
		Name:        name,
		Description: "Custom authentication function",
		Handler:     handler,
		Metadata:    map[string]interface{}{"type": "authentication"},
	}, handler
}

// TransformFunction creates a request/response transformation user function
func TransformFunction(name string, transformer func(*gin.Context)) (*UserFunction, gin.HandlerFunc) {
	handler := func(c *gin.Context) {
		transformer(c)
		c.Next()
	}
	
	return &UserFunction{
		Name:        name,
		Description: "Request/response transformation function",
		Handler:     handler,
		Metadata:    map[string]interface{}{"type": "transform"},
	}, handler
}
