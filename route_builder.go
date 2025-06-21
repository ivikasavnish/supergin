package supergin

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
)

// RouteBuilder provides a fluent interface for building routes
type RouteBuilder struct {
	engine      *Engine
	name        string
	method      string
	path        string
	handler     gin.HandlerFunc
	inputType   reflect.Type
	outputType  reflect.Type
	metadata    map[string]interface{}
	description string
	tags        []string
	middleware  []gin.HandlerFunc
}

// Named creates a new route builder with a name
func (e *Engine) Named(name string) *RouteBuilder {
	return &RouteBuilder{
		engine:     e,
		name:       name,
		metadata:   make(map[string]interface{}),
		middleware: []gin.HandlerFunc{},
	}
}

// GET sets the HTTP method to GET
func (rb *RouteBuilder) GET(path string) *RouteBuilder {
	rb.method = "GET"
	rb.path = path
	return rb
}

// POST sets the HTTP method to POST
func (rb *RouteBuilder) POST(path string) *RouteBuilder {
	rb.method = "POST"
	rb.path = path
	return rb
}

// PUT sets the HTTP method to PUT
func (rb *RouteBuilder) PUT(path string) *RouteBuilder {
	rb.method = "PUT"
	rb.path = path
	return rb
}

// DELETE sets the HTTP method to DELETE
func (rb *RouteBuilder) DELETE(path string) *RouteBuilder {
	rb.method = "DELETE"
	rb.path = path
	return rb
}

// PATCH sets the HTTP method to PATCH
func (rb *RouteBuilder) PATCH(path string) *RouteBuilder {
	rb.method = "PATCH"
	rb.path = path
	return rb
}

// WithIO sets input and output types for validation
func (rb *RouteBuilder) WithIO(input, output interface{}) *RouteBuilder {
	if input != nil {
		rb.inputType = reflect.TypeOf(input)
	}
	if output != nil {
		rb.outputType = reflect.TypeOf(output)
	}
	return rb
}

// WithInput sets only the input type for validation
func (rb *RouteBuilder) WithInput(input interface{}) *RouteBuilder {
	if input != nil {
		rb.inputType = reflect.TypeOf(input)
	}
	return rb
}

// WithOutput sets only the output type for validation
func (rb *RouteBuilder) WithOutput(output interface{}) *RouteBuilder {
	if output != nil {
		rb.outputType = reflect.TypeOf(output)
	}
	return rb
}

// WithMetadata adds metadata to the route
func (rb *RouteBuilder) WithMetadata(key string, value interface{}) *RouteBuilder {
	rb.metadata[key] = value
	return rb
}

// WithDescription adds a description to the route
func (rb *RouteBuilder) WithDescription(desc string) *RouteBuilder {
	rb.description = desc
	return rb
}

// WithTags adds tags to the route
func (rb *RouteBuilder) WithTags(tags ...string) *RouteBuilder {
	rb.tags = append(rb.tags, tags...)
	return rb
}

// WithMiddleware adds middleware to the route
func (rb *RouteBuilder) WithMiddleware(middleware ...gin.HandlerFunc) *RouteBuilder {
	rb.middleware = append(rb.middleware, middleware...)
	return rb
}

// Handler sets the handler function and registers the route
func (rb *RouteBuilder) Handler(handler gin.HandlerFunc) *RouteBuilder {
	rb.handler = handler
	rb.register()
	return rb
}

// HandlerFunc is an alias for Handler for convenience
func (rb *RouteBuilder) HandlerFunc(handler gin.HandlerFunc) *RouteBuilder {
	return rb.Handler(handler)
}

// register actually registers the route with gin and stores metadata
func (rb *RouteBuilder) register() {
	if rb.name == "" {
		panic("route name is required")
	}
	if rb.method == "" {
		panic("HTTP method is required")
	}
	if rb.path == "" {
		panic("route path is required")
	}
	if rb.handler == nil {
		panic("handler function is required")
	}

	// Create enhanced handler with validation
	enhancedHandler := rb.createEnhancedHandler()

	// Combine middleware with enhanced handler
	handlers := append(rb.middleware, enhancedHandler)

	// Register with gin
	switch rb.method {
	case "GET":
		rb.engine.Engine.GET(rb.path, handlers...)
	case "POST":
		rb.engine.Engine.POST(rb.path, handlers...)
	case "PUT":
		rb.engine.Engine.PUT(rb.path, handlers...)
	case "DELETE":
		rb.engine.Engine.DELETE(rb.path, handlers...)
	case "PATCH":
		rb.engine.Engine.PATCH(rb.path, handlers...)
	default:
		panic(fmt.Sprintf("unsupported HTTP method: %s", rb.method))
	}

	// Store route info
	rb.engine.routesMux.Lock()
	rb.engine.routes[rb.name] = &RouteInfo{
		Name:        rb.name,
		Method:      rb.method,
		Path:        rb.path,
		Handler:     rb.handler,
		InputType:   rb.inputType,
		OutputType:  rb.outputType,
		Metadata:    rb.metadata,
		Description: rb.description,
		Tags:        rb.tags,
		CreatedAt:   time.Now(),
	}
	rb.engine.routesMux.Unlock()
}

// createEnhancedHandler wraps the original handler with validation
func (rb *RouteBuilder) createEnhancedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Input validation
		if rb.engine.config.ValidateInput && rb.inputType != nil {
			if err := rb.validateInput(c); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Input validation failed",
					"details": err.Error(),
				})
				return
			}
		}

		// Call original handler
		rb.handler(c)

		// Output validation (if enabled and response is JSON)
		if rb.engine.config.ValidateOutput && rb.outputType != nil {
			rb.validateOutput(c)
		}
	}
}

// validateInput validates the request input
func (rb *RouteBuilder) validateInput(c *gin.Context) error {
	// Create new instance of input type
	inputValue := reflect.New(rb.inputType).Interface()

	// Bind request data based on content type and method
	var err error
	contentType := c.GetHeader("Content-Type")

	if rb.method == "GET" || rb.method == "DELETE" {
		// For GET/DELETE, bind query parameters
		err = c.ShouldBindQuery(inputValue)
	} else if contentType == "application/x-www-form-urlencoded" || contentType == "multipart/form-data" {
		// For form data
		err = c.ShouldBind(inputValue)
	} else {
		// Default to JSON binding
		err = c.ShouldBindJSON(inputValue)
	}

	if err != nil {
		return NewSuperGinError(ErrValidationFailed, "binding error: %v", err)
	}

	// Validate using validator
	if err := rb.engine.validator.Struct(inputValue); err != nil {
		return NewSuperGinError(ErrValidationFailed, "validation error: %v", err)
	}

	// Store validated input in context for handler use
	c.Set("validated_input", inputValue)
	return nil
}

// validateOutput validates the response output (basic implementation)
func (rb *RouteBuilder) validateOutput(c *gin.Context) {
	// This would require intercepting the response writer
	// Implementation depends on specific requirements
	// For now, we'll skip actual validation but log the capability
}
