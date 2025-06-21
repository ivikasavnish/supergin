// Package supergin provides an enhanced Gin framework with named routes,
// input/output validation, dependency injection, and Rails-like REST resources.
package supergin

import (
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Engine wraps gin.Engine with enhanced capabilities
type Engine struct {
	*gin.Engine
	routes    map[string]*RouteInfo
	routesMux sync.RWMutex
	validator *validator.Validate
	config    Config
	di        *DIContainer
}

// Config holds configuration for SuperGin
type Config struct {
	EnableDocs     bool
	ValidateInput  bool
	ValidateOutput bool
	DocsPath       string
}

// RouteInfo holds metadata about a route
type RouteInfo struct {
	Name        string                 `json:"name"`
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Handler     gin.HandlerFunc        `json:"-"`
	InputType   reflect.Type           `json:"-"`
	OutputType  reflect.Type           `json:"-"`
	Metadata    map[string]interface{} `json:"metadata"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
	CreatedAt   time.Time              `json:"created_at"`
}

// InputOutput defines the container for request/response validation
type InputOutput struct {
	Input  interface{}
	Output interface{}
}

// New creates a new SuperGin engine
func New(config ...Config) *Engine {
	cfg := Config{
		EnableDocs:     true,
		ValidateInput:  true,
		ValidateOutput: false,
		DocsPath:       "/docs",
	}
	if len(config) > 0 {
		cfg = config[0]
	}

	engine := &Engine{
		Engine:    gin.New(),
		routes:    make(map[string]*RouteInfo),
		validator: validator.New(),
		config:    cfg,
		di:        GetDI(),
	}

	// Add built-in middleware
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	
	// Add DI middleware
	engine.Use(engine.di.Middleware())

	// Setup docs endpoint if enabled
	if cfg.EnableDocs {
		engine.setupDocsEndpoint()
	}

	return engine
}

// DI returns the dependency injection container
func (e *Engine) DI() *DIContainer {
	return e.di
}

// GetRoute returns route information by name
func (e *Engine) GetRoute(name string) (*RouteInfo, bool) {
	e.routesMux.RLock()
	defer e.routesMux.RUnlock()
	route, exists := e.routes[name]
	return route, exists
}

// GetRoutes returns all registered routes
func (e *Engine) GetRoutes() map[string]*RouteInfo {
	e.routesMux.RLock()
	defer e.routesMux.RUnlock()
	
	// Create a copy to avoid race conditions
	routes := make(map[string]*RouteInfo)
	for k, v := range e.routes {
		routes[k] = v
	}
	return routes
}

// GetRoutesByTag returns routes filtered by tag
func (e *Engine) GetRoutesByTag(tag string) []*RouteInfo {
	e.routesMux.RLock()
	defer e.routesMux.RUnlock()
	
	var routes []*RouteInfo
	for _, route := range e.routes {
		for _, t := range route.Tags {
			if t == tag {
				routes = append(routes, route)
				break
			}
		}
	}
	return routes
}

// URLFor generates URL for a named route with parameters
func (e *Engine) URLFor(name string, params ...interface{}) (string, error) {
	route, exists := e.GetRoute(name)
	if !exists {
		return "", NewSuperGinError(ErrRouteNotFound, "route '%s' not found", name)
	}

	url := route.Path
	
	// Simple parameter replacement (basic implementation)
	for i := 0; i < len(params); i += 2 {
		if i+1 < len(params) {
			key := ":" + params[i].(string)
			value := params[i+1].(string)
			url = strings.Replace(url, key, value, 1)
		}
	}
	
	return url, nil
}

// setupDocsEndpoint creates an endpoint for API documentation
func (e *Engine) setupDocsEndpoint() {
	e.Engine.GET(e.config.DocsPath, func(c *gin.Context) {
		routes := e.GetRoutes()
		
		// Convert to JSON-serializable format
		docs := map[string]interface{}{
			"routes":       routes,
			"generated_at": time.Now(),
			"total_routes": len(routes),
			"di_services":  e.di.ListServices(),
		}
		
		c.JSON(http.StatusOK, docs)
	})
}

// GetValidatedInput retrieves validated input from context
func GetValidatedInput(c *gin.Context) (interface{}, bool) {
	input, exists := c.Get("validated_input")
	return input, exists
}