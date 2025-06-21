package supergin

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
)

// CRUDController interface for REST operations
type CRUDController interface {
	Create(c *gin.Context)
	Read(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	List(c *gin.Context)
	Search(c *gin.Context)
}

// ModelInfo holds information about a model for route generation
type ModelInfo struct {
	Name         string
	PluralName   string
	BasePath     string
	Controller   CRUDController
	InputType    reflect.Type
	OutputType   reflect.Type
	SearchType   reflect.Type
	Middleware   []gin.HandlerFunc
	Tags         []string
	Metadata     map[string]interface{}
	CustomRoutes map[string]CustomRoute
}

// CustomRoute defines additional routes for a model
type CustomRoute struct {
	Method      string
	Path        string
	Handler     gin.HandlerFunc
	Name        string
	Description string
	InputType   reflect.Type
	OutputType  reflect.Type
}

// RestRoutes holds the generated REST route names
type RestRoutes struct {
	Create string
	Read   string
	Update string
	Delete string
	List   string
	Search string
}

// ResourceBuilder provides Rails-like resource routing
type ResourceBuilder struct {
	engine     *Engine
	modelInfo  *ModelInfo
	restRoutes *RestRoutes
}

// Resource creates a new resource builder for a model
func (e *Engine) Resource(name string, controller CRUDController) *ResourceBuilder {
	pluralName := pluralize(name)
	basePath := "/" + strings.ToLower(pluralName)
	
	modelInfo := &ModelInfo{
		Name:         name,
		PluralName:   pluralName,
		BasePath:     basePath,
		Controller:   controller,
		Middleware:   []gin.HandlerFunc{},
		Tags:         []string{strings.ToLower(name)},
		Metadata:     make(map[string]interface{}),
		CustomRoutes: make(map[string]CustomRoute),
	}

	return &ResourceBuilder{
		engine:    e,
		modelInfo: modelInfo,
		restRoutes: &RestRoutes{
			Create: fmt.Sprintf("create_%s", strings.ToLower(name)),
			Read:   fmt.Sprintf("show_%s", strings.ToLower(name)),
			Update: fmt.Sprintf("update_%s", strings.ToLower(name)),
			Delete: fmt.Sprintf("delete_%s", strings.ToLower(name)),
			List:   fmt.Sprintf("list_%s", strings.ToLower(pluralName)),
			Search: fmt.Sprintf("search_%s", strings.ToLower(pluralName)),
		},
	}
}

// WithModel attaches model types to the resource
func (rb *ResourceBuilder) WithModel(input, output, search interface{}) *ResourceBuilder {
	if input != nil {
		rb.modelInfo.InputType = reflect.TypeOf(input)
	}
	if output != nil {
		rb.modelInfo.OutputType = reflect.TypeOf(output)
	}
	if search != nil {
		rb.modelInfo.SearchType = reflect.TypeOf(search)
	}
	return rb
}

// WithMiddleware adds middleware to all resource routes
func (rb *ResourceBuilder) WithMiddleware(middleware ...gin.HandlerFunc) *ResourceBuilder {
	rb.modelInfo.Middleware = append(rb.modelInfo.Middleware, middleware...)
	return rb
}

// WithTags adds tags to all resource routes
func (rb *ResourceBuilder) WithTags(tags ...string) *ResourceBuilder {
	rb.modelInfo.Tags = append(rb.modelInfo.Tags, tags...)
	return rb
}

// WithBasePath sets a custom base path for the resource
func (rb *ResourceBuilder) WithBasePath(path string) *ResourceBuilder {
	rb.modelInfo.BasePath = path
	return rb
}

// WithMetadata adds metadata to all resource routes
func (rb *ResourceBuilder) WithMetadata(key string, value interface{}) *ResourceBuilder {
	rb.modelInfo.Metadata[key] = value
	return rb
}

// Member adds a custom member route (operates on a single resource)
func (rb *ResourceBuilder) Member(name, method, path string, handler gin.HandlerFunc) *ResourceBuilder {
	fullPath := rb.modelInfo.BasePath + "/:id" + path
	routeName := fmt.Sprintf("%s_%s", strings.ToLower(rb.modelInfo.Name), name)
	
	rb.modelInfo.CustomRoutes[name] = CustomRoute{
		Method:      method,
		Path:        fullPath,
		Handler:     handler,
		Name:        routeName,
		Description: fmt.Sprintf("%s %s", name, rb.modelInfo.Name),
	}
	return rb
}

// Collection adds a custom collection route (operates on the collection)
func (rb *ResourceBuilder) Collection(name, method, path string, handler gin.HandlerFunc) *ResourceBuilder {
	fullPath := rb.modelInfo.BasePath + path
	routeName := fmt.Sprintf("%s_%s", strings.ToLower(rb.modelInfo.PluralName), name)
	
	rb.modelInfo.CustomRoutes[name] = CustomRoute{
		Method:      method,
		Path:        fullPath,
		Handler:     handler,
		Name:        routeName,
		Description: fmt.Sprintf("%s %s", name, rb.modelInfo.PluralName),
	}
	return rb
}

// Only restricts which REST actions to generate
func (rb *ResourceBuilder) Only(actions ...string) *ResourceBuilder {
	// Store which actions to generate
	rb.modelInfo.Metadata["only_actions"] = actions
	return rb
}

// Except excludes specific REST actions from generation
func (rb *ResourceBuilder) Except(actions ...string) *ResourceBuilder {
	// Store which actions to exclude
	rb.modelInfo.Metadata["except_actions"] = actions
	return rb
}

// Build generates all the REST routes and custom routes
func (rb *ResourceBuilder) Build() *RestRoutes {
	onlyActions, hasOnly := rb.modelInfo.Metadata["only_actions"].([]string)
	exceptActions, hasExcept := rb.modelInfo.Metadata["except_actions"].([]string)
	
	shouldGenerate := func(action string) bool {
		if hasOnly {
			return contains(onlyActions, action)
		}
		if hasExcept {
			return !contains(exceptActions, action)
		}
		return true
	}

	// Generate standard REST routes
	if shouldGenerate("list") {
		rb.generateListRoute()
	}
	if shouldGenerate("create") {
		rb.generateCreateRoute()
	}
	if shouldGenerate("read") {
		rb.generateReadRoute()
	}
	if shouldGenerate("update") {
		rb.generateUpdateRoute()
	}
	if shouldGenerate("delete") {
		rb.generateDeleteRoute()
	}
	if shouldGenerate("search") {
		rb.generateSearchRoute()
	}

	// Generate custom routes
	for _, customRoute := range rb.modelInfo.CustomRoutes {
		rb.generateCustomRoute(customRoute)
	}

	return rb.restRoutes
}

// Generate individual REST routes
func (rb *ResourceBuilder) generateListRoute() {
	builder := rb.engine.Named(rb.restRoutes.List).
		GET(rb.modelInfo.BasePath).
		WithDescription(fmt.Sprintf("List all %s", rb.modelInfo.PluralName)).
		WithTags(rb.modelInfo.Tags...).
		WithMiddleware(rb.modelInfo.Middleware...)

	if rb.modelInfo.OutputType != nil {
		// For list, we expect a slice of the output type
		sliceType := reflect.SliceOf(rb.modelInfo.OutputType)
		builder.WithOutput(reflect.New(sliceType).Elem().Interface())
	}

	for k, v := range rb.modelInfo.Metadata {
		builder.WithMetadata(k, v)
	}

	builder.Handler(rb.modelInfo.Controller.List)
}

func (rb *ResourceBuilder) generateCreateRoute() {
	builder := rb.engine.Named(rb.restRoutes.Create).
		POST(rb.modelInfo.BasePath).
		WithDescription(fmt.Sprintf("Create a new %s", rb.modelInfo.Name)).
		WithTags(rb.modelInfo.Tags...).
		WithMiddleware(rb.modelInfo.Middleware...)

	if rb.modelInfo.InputType != nil && rb.modelInfo.OutputType != nil {
		builder.WithIO(
			reflect.New(rb.modelInfo.InputType).Elem().Interface(),
			reflect.New(rb.modelInfo.OutputType).Elem().Interface(),
		)
	}

	for k, v := range rb.modelInfo.Metadata {
		builder.WithMetadata(k, v)
	}

	builder.Handler(rb.modelInfo.Controller.Create)
}

func (rb *ResourceBuilder) generateReadRoute() {
	builder := rb.engine.Named(rb.restRoutes.Read).
		GET(rb.modelInfo.BasePath + "/:id").
		WithDescription(fmt.Sprintf("Get %s by ID", rb.modelInfo.Name)).
		WithTags(rb.modelInfo.Tags...).
		WithMiddleware(rb.modelInfo.Middleware...)

	if rb.modelInfo.OutputType != nil {
		builder.WithOutput(reflect.New(rb.modelInfo.OutputType).Elem().Interface())
	}

	for k, v := range rb.modelInfo.Metadata {
		builder.WithMetadata(k, v)
	}

	builder.Handler(rb.modelInfo.Controller.Read)
}

func (rb *ResourceBuilder) generateUpdateRoute() {
	builder := rb.engine.Named(rb.restRoutes.Update).
		PUT(rb.modelInfo.BasePath + "/:id").
		WithDescription(fmt.Sprintf("Update %s by ID", rb.modelInfo.Name)).
		WithTags(rb.modelInfo.Tags...).
		WithMiddleware(rb.modelInfo.Middleware...)

	if rb.modelInfo.InputType != nil && rb.modelInfo.OutputType != nil {
		builder.WithIO(
			reflect.New(rb.modelInfo.InputType).Elem().Interface(),
			reflect.New(rb.modelInfo.OutputType).Elem().Interface(),
		)
	}

	for k, v := range rb.modelInfo.Metadata {
		builder.WithMetadata(k, v)
	}

	builder.Handler(rb.modelInfo.Controller.Update)
}

func (rb *ResourceBuilder) generateDeleteRoute() {
	builder := rb.engine.Named(rb.restRoutes.Delete).
		DELETE(rb.modelInfo.BasePath + "/:id").
		WithDescription(fmt.Sprintf("Delete %s by ID", rb.modelInfo.Name)).
		WithTags(rb.modelInfo.Tags...).
		WithMiddleware(rb.modelInfo.Middleware...)

	for k, v := range rb.modelInfo.Metadata {
		builder.WithMetadata(k, v)
	}

	builder.Handler(rb.modelInfo.Controller.Delete)
}

func (rb *ResourceBuilder) generateSearchRoute() {
	builder := rb.engine.Named(rb.restRoutes.Search).
		GET(rb.modelInfo.BasePath + "/search").
		WithDescription(fmt.Sprintf("Search %s", rb.modelInfo.PluralName)).
		WithTags(rb.modelInfo.Tags...).
		WithMiddleware(rb.modelInfo.Middleware...)

	if rb.modelInfo.SearchType != nil && rb.modelInfo.OutputType != nil {
		sliceType := reflect.SliceOf(rb.modelInfo.OutputType)
		builder.WithIO(
			reflect.New(rb.modelInfo.SearchType).Elem().Interface(),
			reflect.New(sliceType).Elem().Interface(),
		)
	}

	for k, v := range rb.modelInfo.Metadata {
		builder.WithMetadata(k, v)
	}

	builder.Handler(rb.modelInfo.Controller.Search)
}

func (rb *ResourceBuilder) generateCustomRoute(customRoute CustomRoute) {
	builder := rb.engine.Named(customRoute.Name)

	switch customRoute.Method {
	case "GET":
		builder.GET(customRoute.Path)
	case "POST":
		builder.POST(customRoute.Path)
	case "PUT":
		builder.PUT(customRoute.Path)
	case "DELETE":
		builder.DELETE(customRoute.Path)
	case "PATCH":
		builder.PATCH(customRoute.Path)
	}

	builder.WithDescription(customRoute.Description).
		WithTags(rb.modelInfo.Tags...).
		WithMiddleware(rb.modelInfo.Middleware...)

	if customRoute.InputType != nil && customRoute.OutputType != nil {
		builder.WithIO(
			reflect.New(customRoute.InputType).Elem().Interface(),
			reflect.New(customRoute.OutputType).Elem().Interface(),
		)
	}

	for k, v := range rb.modelInfo.Metadata {
		builder.WithMetadata(k, v)
	}

	builder.Handler(customRoute.Handler)
}

// Helper functions
func pluralize(word string) string {
	// Simple pluralization - you might want to use a proper library
	if strings.HasSuffix(word, "y") {
		return strings.TrimSuffix(word, "y") + "ies"
	}
	if strings.HasSuffix(word, "s") || strings.HasSuffix(word, "x") || strings.HasSuffix(word, "z") {
		return word + "es"
	}
	return word + "s"
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}