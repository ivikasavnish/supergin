package supergin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// GrpcConverter interface for types that can convert to/from gRPC
type GrpcConverter interface {
	ToGrpc() (proto.Message, error)
	FromGrpc(proto.Message) error
}

// GrpcService represents a gRPC service configuration
type GrpcService struct {
	Name        string
	Address     string
	ServiceName string
	Methods     map[string]*GrpcMethod
	Connection  *grpc.ClientConn
}

// GrpcMethod represents a gRPC method configuration
type GrpcMethod struct {
	Name            string
	FullName        string
	InputType       reflect.Type
	OutputType      reflect.Type
	GrpcInputType   reflect.Type
	GrpcOutputType  reflect.Type
	StreamingInput  bool
	StreamingOutput bool
}

// GrpcBridge manages HTTP to gRPC conversions
type GrpcBridge struct {
	services map[string]*GrpcService
	engine   *Engine
}

// NewGrpcBridge creates a new gRPC bridge
func NewGrpcBridge(engine *Engine) *GrpcBridge {
	return &GrpcBridge{
		services: make(map[string]*GrpcService),
		engine:   engine,
	}
}

// RegisterGrpcService registers a gRPC service for HTTP bridging
func (gb *GrpcBridge) RegisterGrpcService(name, address, serviceName string) error {
	// Create gRPC connection
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC service %s at %s: %v", name, address, err)
	}

	service := &GrpcService{
		Name:        name,
		Address:     address,
		ServiceName: serviceName,
		Methods:     make(map[string]*GrpcMethod),
		Connection:  conn,
	}

	gb.services[name] = service
	return nil
}

// RegisterGrpcMethod registers a gRPC method with type mappings
func (gb *GrpcBridge) RegisterGrpcMethod(serviceName, methodName string,
	httpInputType, httpOutputType, grpcInputType, grpcOutputType interface{}) error {

	service, exists := gb.services[serviceName]
	if !exists {
		return fmt.Errorf("gRPC service %s not found", serviceName)
	}

	method := &GrpcMethod{
		Name:           methodName,
		FullName:       fmt.Sprintf("/%s/%s", service.ServiceName, methodName),
		InputType:      reflect.TypeOf(httpInputType),
		OutputType:     reflect.TypeOf(httpOutputType),
		GrpcInputType:  reflect.TypeOf(grpcInputType),
		GrpcOutputType: reflect.TypeOf(grpcOutputType),
	}

	service.Methods[methodName] = method
	return nil
}

// Engine extension for gRPC bridge
func (e *Engine) GrpcBridge() *GrpcBridge {
	if bridge, exists := e.di.Get("grpc_bridge").(*GrpcBridge); exists {
		return bridge
	}

	bridge := NewGrpcBridge(e)
	e.di.RegisterInstance("grpc_bridge", bridge)
	return bridge
}

// Route builder extension for gRPC bridging
func (rb *RouteBuilder) WithGrpcBridge(serviceName, methodName string) *RouteBuilder {
	rb.WithMetadata("grpc_service", serviceName)
	rb.WithMetadata("grpc_method", methodName)

	// Create the bridging handler
	originalHandler := rb.handler
	rb.handler = func(c *gin.Context) {
		bridge := rb.engine.GrpcBridge()

		// Handle gRPC bridging
		if err := bridge.handleHttpToGrpc(c, serviceName, methodName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "gRPC bridge error",
				"details": err.Error(),
			})
			return
		}

		// Call original handler if needed
		if originalHandler != nil {
			originalHandler(c)
		}
	}

	return rb
}

// handleHttpToGrpc handles HTTP to gRPC conversion
func (gb *GrpcBridge) handleHttpToGrpc(c *gin.Context, serviceName, methodName string) error {
	service, exists := gb.services[serviceName]
	if !exists {
		return fmt.Errorf("gRPC service %s not found", serviceName)
	}

	method, exists := service.Methods[methodName]
	if !exists {
		return fmt.Errorf("gRPC method %s not found in service %s", methodName, serviceName)
	}

	// Get validated HTTP input
	var httpInput interface{}
	if input, exists := GetValidatedInput(c); exists {
		httpInput = input
	} else {
		// Create new instance and bind
		httpInput = reflect.New(method.InputType).Interface()
		if err := c.ShouldBindJSON(httpInput); err != nil {
			return fmt.Errorf("failed to bind HTTP input: %v", err)
		}
	}

	// Convert HTTP input to gRPC input
	grpcInput, err := gb.convertToGrpc(httpInput, method.GrpcInputType)
	if err != nil {
		return fmt.Errorf("failed to convert HTTP input to gRPC: %v", err)
	}

	// Make gRPC call
	grpcOutput, err := gb.callGrpcMethod(c.Request.Context(), service, method, grpcInput)
	if err != nil {
		return fmt.Errorf("gRPC call failed: %v", err)
	}

	// Convert gRPC output to HTTP output
	httpOutput, err := gb.convertFromGrpc(grpcOutput, method.OutputType)
	if err != nil {
		return fmt.Errorf("failed to convert gRPC output to HTTP: %v", err)
	}

	// Send HTTP response
	c.JSON(http.StatusOK, httpOutput)
	return nil
}

// convertToGrpc converts HTTP input to gRPC message
func (gb *GrpcBridge) convertToGrpc(httpInput interface{}, grpcType reflect.Type) (proto.Message, error) {
	// Check if input implements GrpcConverter
	if converter, ok := httpInput.(GrpcConverter); ok {
		return converter.ToGrpc()
	}

	// Generic conversion via JSON marshaling/unmarshaling
	httpJSON, err := json.Marshal(httpInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal HTTP input: %v", err)
	}

	// Create new gRPC message instance
	grpcValue := reflect.New(grpcType.Elem()).Interface()
	grpcMsg, ok := grpcValue.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("gRPC type %s does not implement proto.Message", grpcType)
	}

	// Convert JSON to protobuf
	if err := protojson.Unmarshal(httpJSON, grpcMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to protobuf: %v", err)
	}

	return grpcMsg, nil
}

// convertFromGrpc converts gRPC message to HTTP output
func (gb *GrpcBridge) convertFromGrpc(grpcOutput proto.Message, httpType reflect.Type) (interface{}, error) {
	// Create new HTTP output instance
	httpValue := reflect.New(httpType)
	httpOutput := httpValue.Interface()

	// Check if output implements GrpcConverter
	if converter, ok := httpOutput.(GrpcConverter); ok {
		if err := converter.FromGrpc(grpcOutput); err != nil {
			return nil, err
		}
		return httpOutput, nil
	}

	// Generic conversion via protobuf/JSON marshaling
	grpcJSON, err := protojson.Marshal(grpcOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf to JSON: %v", err)
	}

	// Unmarshal JSON to HTTP output
	if err := json.Unmarshal(grpcJSON, httpOutput); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to HTTP output: %v", err)
	}

	return httpOutput, nil
}

// callGrpcMethod makes the actual gRPC call
func (gb *GrpcBridge) callGrpcMethod(ctx context.Context, service *GrpcService, method *GrpcMethod, input proto.Message) (proto.Message, error) {
	// Create gRPC output message instance
	outputValue := reflect.New(method.GrpcOutputType.Elem()).Interface()
	output, ok := outputValue.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("gRPC output type does not implement proto.Message")
	}

	// Prepare gRPC metadata from HTTP headers
	md := metadata.New(nil)

	// Make the gRPC call using the generic Invoke method
	err := service.Connection.Invoke(ctx, method.FullName, input, output, grpc.Header(&md))
	if err != nil {
		return nil, err
	}

	return output, nil
}

// Reverse proxy: gRPC to HTTP
func (gb *GrpcBridge) CreateGrpcToHttpProxy(serviceName, methodName string, httpEndpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		service, exists := gb.services[serviceName]
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gRPC service not found"})
			return
		}

		method, exists := service.Methods[methodName]
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gRPC method not found"})
			return
		}

		// Read gRPC request (assuming protobuf in request body)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		// Create gRPC input message
		grpcInputValue := reflect.New(method.GrpcInputType.Elem()).Interface()
		grpcInput, ok := grpcInputValue.(proto.Message)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid gRPC input type"})
			return
		}

		// Unmarshal protobuf
		if err := proto.Unmarshal(body, grpcInput); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to unmarshal protobuf"})
			return
		}

		// Convert to HTTP format
		httpInput, err := gb.convertFromGrpc(grpcInput, method.InputType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Make HTTP call
		httpResponse, err := gb.makeHttpCall(httpEndpoint, httpInput)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Convert HTTP response back to gRPC
		grpcOutput, err := gb.convertToGrpc(httpResponse, method.GrpcOutputType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Marshal and send protobuf response
		grpcBytes, err := proto.Marshal(grpcOutput)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal protobuf"})
			return
		}

		c.Data(http.StatusOK, "application/x-protobuf", grpcBytes)
	}
}

// makeHttpCall makes an HTTP call to the specified endpoint
func (gb *GrpcBridge) makeHttpCall(endpoint string, input interface{}) (interface{}, error) {
	// Marshal input to JSON
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %v", err)
	}

	// Make HTTP POST request
	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse JSON response
	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return result, nil
}

// Helper function to register both HTTP and gRPC endpoints
func (e *Engine) BidirectionalGrpcHttp(name string, httpPath string, grpcService string, grpcMethod string,
	httpInput, httpOutput, grpcInput, grpcOutput interface{}) error {

	bridge := e.GrpcBridge()

	// Register the gRPC method mapping
	err := bridge.RegisterGrpcMethod(grpcService, grpcMethod, httpInput, httpOutput, grpcInput, grpcOutput)
	if err != nil {
		return err
	}

	// Create HTTP endpoint that bridges to gRPC
	e.Named(name+"_http_to_grpc").
		POST(httpPath).
		WithIO(httpInput, httpOutput).
		WithDescription(fmt.Sprintf("HTTP to gRPC bridge for %s", name)).
		WithTags("grpc", "bridge").
		WithGrpcBridge(grpcService, grpcMethod).
		Handler(func(c *gin.Context) {
			// Handler is set up by WithGrpcBridge
		})

	// Create reverse gRPC endpoint that bridges to HTTP
	reverseHttpPath := strings.Replace(httpPath, "/api/", "/grpc/", 1)
	e.Named(name+"_grpc_to_http").
		POST(reverseHttpPath).
		WithDescription(fmt.Sprintf("gRPC to HTTP bridge for %s", name)).
		WithTags("grpc", "bridge", "reverse").
		Handler(bridge.CreateGrpcToHttpProxy(grpcService, grpcMethod, "http://localhost:8080"+httpPath))

	return nil
}
