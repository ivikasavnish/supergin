package supergin

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/gin-gonic/gin"
)

// DIScope defines the lifecycle of a service
type DIScope string

const (
	ScopeSingleton DIScope = "singleton" // One instance for entire app
	ScopeRequest   DIScope = "request"   // One instance per HTTP request
	ScopeTransient DIScope = "transient" // New instance every time
)

// ServiceDefinition defines how to create and manage a service
type ServiceDefinition struct {
	Name         string                 `json:"name"`
	Type         reflect.Type           `json:"-"`
	Scope        DIScope                `json:"scope"`
	Factory      interface{}            `json:"-"`
	Dependencies []string               `json:"dependencies"`
	Singleton    interface{}            `json:"-"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// DIContainer manages dependency injection
type DIContainer struct {
	services   map[string]*ServiceDefinition
	singletons map[string]interface{}
	mutex      sync.RWMutex
	requestKey string
}

// RequestScope holds request-scoped dependencies
type RequestScope struct {
	instances map[string]interface{}
	mutex     sync.RWMutex
}

// Global DI container instance
var globalDI *DIContainer
var diOnce sync.Once

// GetDI returns the global DI container
func GetDI() *DIContainer {
	diOnce.Do(func() {
		globalDI = &DIContainer{
			services:   make(map[string]*ServiceDefinition),
			singletons: make(map[string]interface{}),
			requestKey: "supergin:request_scope",
		}
	})
	return globalDI
}

// Register registers a service with the DI container
func (di *DIContainer) Register(name string, factory interface{}, scope DIScope, dependencies ...string) *DIContainer {
	factoryType := reflect.TypeOf(factory)
	if factoryType.Kind() != reflect.Func {
		panic(fmt.Sprintf("factory for service '%s' must be a function", name))
	}

	// Validate factory function returns exactly one value
	if factoryType.NumOut() != 1 {
		panic(fmt.Sprintf("factory for service '%s' must return exactly one value", name))
	}

	di.mutex.Lock()
	defer di.mutex.Unlock()

	di.services[name] = &ServiceDefinition{
		Name:         name,
		Type:         factoryType.Out(0),
		Scope:        scope,
		Factory:      factory,
		Dependencies: dependencies,
		Metadata:     make(map[string]interface{}),
	}

	return di
}

// RegisterSingleton registers a singleton service
func (di *DIContainer) RegisterSingleton(name string, factory interface{}, dependencies ...string) *DIContainer {
	return di.Register(name, factory, ScopeSingleton, dependencies...)
}

// RegisterRequest registers a request-scoped service
func (di *DIContainer) RegisterRequest(name string, factory interface{}, dependencies ...string) *DIContainer {
	return di.Register(name, factory, ScopeRequest, dependencies...)
}

// RegisterTransient registers a transient service
func (di *DIContainer) RegisterTransient(name string, factory interface{}, dependencies ...string) *DIContainer {
	return di.Register(name, factory, ScopeTransient, dependencies...)
}

// RegisterInstance registers a pre-created instance as a singleton
func (di *DIContainer) RegisterInstance(name string, instance interface{}) *DIContainer {
	di.mutex.Lock()
	defer di.mutex.Unlock()

	instanceType := reflect.TypeOf(instance)
	di.services[name] = &ServiceDefinition{
		Name:      name,
		Type:      instanceType,
		Scope:     ScopeSingleton,
		Singleton: instance,
		Metadata:  make(map[string]interface{}),
	}
	di.singletons[name] = instance

	return di
}

// Get resolves and returns a service instance
func (di *DIContainer) Get(name string) interface{} {
	return di.resolve(name, make(map[string]bool), nil)
}

// GetFromContext resolves a service with request context
func (di *DIContainer) GetFromContext(ctx context.Context, name string) interface{} {
	return di.resolve(name, make(map[string]bool), ctx)
}

// GetT returns a typed service instance
func GetT[T any](name string) T {
	instance := GetDI().Get(name)
	if instance == nil {
		var zero T
		return zero
	}
	return instance.(T)
}

// GetFromContextT returns a typed service instance with context
func GetFromContextT[T any](ctx context.Context, name string) T {
	instance := GetDI().GetFromContext(ctx, name)
	if instance == nil {
		var zero T
		return zero
	}
	return instance.(T)
}

// resolve internal method to resolve dependencies
func (di *DIContainer) resolve(name string, resolving map[string]bool, ctx context.Context) interface{} {
	// Check for circular dependencies
	if resolving[name] {
		panic(fmt.Sprintf("circular dependency detected for service '%s'", name))
	}
	resolving[name] = true
	defer delete(resolving, name)

	di.mutex.RLock()
	service, exists := di.services[name]
	di.mutex.RUnlock()

	if !exists {
		panic(fmt.Sprintf("service '%s' not registered", name))
	}

	switch service.Scope {
	case ScopeSingleton:
		return di.resolveSingleton(service, resolving, ctx)
	case ScopeRequest:
		return di.resolveRequest(service, resolving, ctx)
	case ScopeTransient:
		return di.resolveTransient(service, resolving, ctx)
	default:
		panic(fmt.Sprintf("unknown scope '%s' for service '%s'", service.Scope, name))
	}
}

func (di *DIContainer) resolveSingleton(service *ServiceDefinition, resolving map[string]bool, ctx context.Context) interface{} {
	// Check if already cached
	if service.Singleton != nil {
		return service.Singleton
	}

	di.mutex.Lock()
	defer di.mutex.Unlock()

	// Double-check after acquiring lock
	if service.Singleton != nil {
		return service.Singleton
	}

	instance := di.createInstance(service, resolving, ctx)
	service.Singleton = instance
	di.singletons[service.Name] = instance
	return instance
}

func (di *DIContainer) resolveRequest(service *ServiceDefinition, resolving map[string]bool, ctx context.Context) interface{} {
	if ctx == nil {
		panic(fmt.Sprintf("request-scoped service '%s' requires context", service.Name))
	}

	// Get or create request scope
	var requestScope *RequestScope
	if ginCtx, ok := ctx.(*gin.Context); ok {
		if scope, exists := ginCtx.Get(di.requestKey); exists {
			requestScope = scope.(*RequestScope)
		} else {
			requestScope = &RequestScope{
				instances: make(map[string]interface{}),
			}
			ginCtx.Set(di.requestKey, requestScope)
		}
	} else {
		// For non-gin contexts, create a new scope
		requestScope = &RequestScope{
			instances: make(map[string]interface{}),
		}
	}

	requestScope.mutex.RLock()
	if instance, exists := requestScope.instances[service.Name]; exists {
		requestScope.mutex.RUnlock()
		return instance
	}
	requestScope.mutex.RUnlock()

	requestScope.mutex.Lock()
	defer requestScope.mutex.Unlock()

	// Double-check after acquiring lock
	if instance, exists := requestScope.instances[service.Name]; exists {
		return instance
	}

	instance := di.createInstance(service, resolving, ctx)
	requestScope.instances[service.Name] = instance
	return instance
}

func (di *DIContainer) resolveTransient(service *ServiceDefinition, resolving map[string]bool, ctx context.Context) interface{} {
	return di.createInstance(service, resolving, ctx)
}

func (di *DIContainer) createInstance(service *ServiceDefinition, resolving map[string]bool, ctx context.Context) interface{} {
	if service.Factory == nil {
		panic(fmt.Sprintf("no factory function for service '%s'", service.Name))
	}

	factoryValue := reflect.ValueOf(service.Factory)
	factoryType := factoryValue.Type()

	// Resolve dependencies
	args := make([]reflect.Value, len(service.Dependencies))
	for i, depName := range service.Dependencies {
		dep := di.resolve(depName, resolving, ctx)
		args[i] = reflect.ValueOf(dep)
	}

	// Validate argument types
	if len(args) != factoryType.NumIn() {
		panic(fmt.Sprintf("service '%s' factory expects %d arguments, got %d dependencies",
			service.Name, factoryType.NumIn(), len(args)))
	}

	// Call factory function
	results := factoryValue.Call(args)
	return results[0].Interface()
}

// Middleware for DI integration
func (di *DIContainer) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create request scope
		requestScope := &RequestScope{
			instances: make(map[string]interface{}),
		}
		c.Set(di.requestKey, requestScope)
		c.Next()
	}
}

// ListServices returns all registered services
func (di *DIContainer) ListServices() map[string]*ServiceDefinition {
	di.mutex.RLock()
	defer di.mutex.RUnlock()

	services := make(map[string]*ServiceDefinition)
	for k, v := range di.services {
		services[k] = v
	}
	return services
}

// Global convenience functions
func Register(name string, factory interface{}, scope DIScope, dependencies ...string) *DIContainer {
	return GetDI().Register(name, factory, scope, dependencies...)
}

func RegisterSingleton(name string, factory interface{}, dependencies ...string) *DIContainer {
	return GetDI().RegisterSingleton(name, factory, dependencies...)
}

func RegisterRequest(name string, factory interface{}, dependencies ...string) *DIContainer {
	return GetDI().RegisterRequest(name, factory, dependencies...)
}

func RegisterTransient(name string, factory interface{}, dependencies ...string) *DIContainer {
	return GetDI().RegisterTransient(name, factory, dependencies...)
}

func RegisterInstance(name string, instance interface{}) *DIContainer {
	return GetDI().RegisterInstance(name, instance)
}

func Get(name string) interface{} {
	return GetDI().Get(name)
}

func GetFromContext(ctx context.Context, name string) interface{} {
	return GetDI().GetFromContext(ctx, name)
}

// Service resolver that works without context in handlers
func Resolve[T any](name string) T {
	// Try to get from current goroutine's gin context if available
	if ginCtx := getCurrentGinContext(); ginCtx != nil {
		return GetFromContextT[T](ginCtx, name)
	}
	return GetT[T](name)
}

// getCurrentGinContext attempts to get gin context from current goroutine
// This is a simplified implementation - in production you'd want a more robust solution
func getCurrentGinContext() *gin.Context {
	// This would require goroutine-local storage implementation
	// For now, we'll return nil and fall back to singleton resolution
	return nil
}
