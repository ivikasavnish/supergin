package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ivikasavnish/supergin"
)

// User models
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

type UserSearchRequest struct {
	Name  string `json:"name,omitempty" form:"name"`
	Email string `json:"email,omitempty" form:"email"`
	Page  int    `json:"page,omitempty" form:"page"`
	Limit int    `json:"limit,omitempty" form:"limit"`
}

// Database layer
type DatabaseConfig struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
}

type Database interface {
	Query(sql string) ([]map[string]interface{}, error)
	Execute(sql string) error
}

type PostgresDB struct {
	config *DatabaseConfig
}

func (db *PostgresDB) Query(sql string) ([]map[string]interface{}, error) {
	// Mock implementation
	fmt.Printf("Executing query: %s\n", sql)
	return []map[string]interface{}{
		{"id": 1, "name": "John Doe", "email": "john@example.com", "age": 30},
		{"id": 2, "name": "Jane Smith", "email": "jane@example.com", "age": 25},
	}, nil
}

func (db *PostgresDB) Execute(sql string) error {
	fmt.Printf("Executing SQL: %s\n", sql)
	return nil
}

// Repository layer
type UserRepository interface {
	FindByID(id int) (*UserResponse, error)
	Create(user *CreateUserRequest) (*UserResponse, error)
	Update(id int, user *CreateUserRequest) (*UserResponse, error)
	Delete(id int) error
	List() ([]*UserResponse, error)
	Search(criteria *UserSearchRequest) ([]*UserResponse, error)
}

type UserRepositoryImpl struct {
	db Database
}

func (r *UserRepositoryImpl) FindByID(id int) (*UserResponse, error) {
	rows, err := r.db.Query(fmt.Sprintf("SELECT * FROM users WHERE id = %d", id))
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return &UserResponse{
		ID:        id,
		Name:      "John Doe",
		Email:     "john@example.com",
		Age:       30,
		CreatedAt: time.Now(),
	}, nil
}

func (r *UserRepositoryImpl) Create(user *CreateUserRequest) (*UserResponse, error) {
	err := r.db.Execute("INSERT INTO users (name, email, age) VALUES ...")
	if err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:        123,
		Name:      user.Name,
		Email:     user.Email,
		Age:       user.Age,
		CreatedAt: time.Now(),
	}, nil
}

func (r *UserRepositoryImpl) Update(id int, user *CreateUserRequest) (*UserResponse, error) {
	err := r.db.Execute(fmt.Sprintf("UPDATE users SET ... WHERE id = %d", id))
	if err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:        id,
		Name:      user.Name,
		Email:     user.Email,
		Age:       user.Age,
		CreatedAt: time.Now(),
	}, nil
}

func (r *UserRepositoryImpl) Delete(id int) error {
	return r.db.Execute(fmt.Sprintf("DELETE FROM users WHERE id = %d", id))
}

func (r *UserRepositoryImpl) List() ([]*UserResponse, error) {
	_, err := r.db.Query("SELECT * FROM users")
	if err != nil {
		return nil, err
	}

	return []*UserResponse{
		{ID: 1, Name: "John Doe", Email: "john@example.com", Age: 30, CreatedAt: time.Now()},
		{ID: 2, Name: "Jane Smith", Email: "jane@example.com", Age: 25, CreatedAt: time.Now()},
	}, nil
}

func (r *UserRepositoryImpl) Search(criteria *UserSearchRequest) ([]*UserResponse, error) {
	query := "SELECT * FROM users WHERE 1=1"
	if criteria.Name != "" {
		query += fmt.Sprintf(" AND name LIKE '%%%s%%'", criteria.Name)
	}
	if criteria.Email != "" {
		query += fmt.Sprintf(" AND email LIKE '%%%s%%'", criteria.Email)
	}

	_, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}

	return []*UserResponse{
		{ID: 1, Name: "John Doe", Email: "john@example.com", Age: 30, CreatedAt: time.Now()},
	}, nil
}

// Service layer
type UserService interface {
	GetUser(id int) (*UserResponse, error)
	CreateUser(user *CreateUserRequest) (*UserResponse, error)
	UpdateUser(id int, user *CreateUserRequest) (*UserResponse, error)
	DeleteUser(id int) error
	ListUsers() ([]*UserResponse, error)
	SearchUsers(criteria *UserSearchRequest) ([]*UserResponse, error)
}

type UserServiceImpl struct {
	repo UserRepository
}

func (s *UserServiceImpl) GetUser(id int) (*UserResponse, error) {
	return s.repo.FindByID(id)
}

func (s *UserServiceImpl) CreateUser(user *CreateUserRequest) (*UserResponse, error) {
	// Add business logic here (validation, transformation, etc.)
	return s.repo.Create(user)
}

func (s *UserServiceImpl) UpdateUser(id int, user *CreateUserRequest) (*UserResponse, error) {
	// Check if user exists first
	_, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(id, user)
}

func (s *UserServiceImpl) DeleteUser(id int) error {
	// Check if user exists first
	_, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	return s.repo.Delete(id)
}

func (s *UserServiceImpl) ListUsers() ([]*UserResponse, error) {
	return s.repo.List()
}

func (s *UserServiceImpl) SearchUsers(criteria *UserSearchRequest) ([]*UserResponse, error) {
	return s.repo.Search(criteria)
}

// Controller layer (using DI)
type UserController struct {
	// No dependencies injected in constructor - resolved at runtime
}

func (uc *UserController) Create(c *gin.Context) {
	// Resolve service without context passing
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

func (uc *UserController) Read(c *gin.Context) {
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

func (uc *UserController) Update(c *gin.Context) {
	userService := supergin.Resolve[UserService]("userService")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if input, exists := supergin.GetValidatedInput(c); exists {
		req := input.(*CreateUserRequest)
		user, err := userService.UpdateUser(id, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

func (uc *UserController) Delete(c *gin.Context) {
	userService := supergin.Resolve[UserService]("userService")

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	err = userService.DeleteUser(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (uc *UserController) List(c *gin.Context) {
	userService := supergin.Resolve[UserService]("userService")

	users, err := userService.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (uc *UserController) Search(c *gin.Context) {
	userService := supergin.Resolve[UserService]("userService")

	if input, exists := supergin.GetValidatedInput(c); exists {
		searchReq := input.(*UserSearchRequest)
		users, err := userService.SearchUsers(searchReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, users)
	}
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

	// Setup routes
	setupRoutes(app)

	// Start server
	fmt.Println("🚀 SuperGin server starting on :8080")
	fmt.Println("📚 API Documentation: http://localhost:8080/api/docs")
	fmt.Println("👥 Users API: http://localhost:8080/users")

	app.Run(":8080")
}

func setupDI() {
	// Register configuration as singleton
	supergin.RegisterInstance("dbConfig", &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "myapp",
		Username: "user",
		Password: "password",
	})

	// Register database as singleton with dependency on config
	supergin.RegisterSingleton("database", func(config *DatabaseConfig) Database {
		fmt.Printf("🔌 Creating database connection to %s:%d/%s\n", config.Host, config.Port, config.Database)
		return &PostgresDB{config: config}
	}, "dbConfig")

	// Register repository as request-scoped with dependency on database
	supergin.RegisterRequest("userRepository", func(db Database) UserRepository {
		return &UserRepositoryImpl{db: db}
	}, "database")

	// Register service as request-scoped with dependency on repository
	supergin.RegisterRequest("userService", func(repo UserRepository) UserService {
		return &UserServiceImpl{repo: repo}
	}, "userRepository")

	fmt.Println("✅ Dependency injection configured")
}

func setupRoutes(app *supergin.Engine) {
	// Create controller
	userController := &UserController{}

	// Generate REST routes for User resource
	userRoutes := app.Resource("User", userController).
		WithModel(CreateUserRequest{}, UserResponse{}, UserSearchRequest{}).
		WithTags("api", "v1", "users").
		WithMetadata("version", "v1").
		WithMetadata("auth_required", false).
		WithMiddleware(func(c *gin.Context) {
			// Request logging middleware
			start := time.Now()
			c.Next()
			duration := time.Since(start)
			fmt.Printf("📝 %s %s - %v\n", c.Request.Method, c.Request.URL.Path, duration)
		}).
		// Add custom member routes
		Member("activate", "POST", "/activate", func(c *gin.Context) {
			userService := supergin.Resolve[UserService]("userService")
			idStr := c.Param("id")
			id, _ := strconv.Atoi(idStr)

			// Simulate activation
			user, err := userService.GetUser(id)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("User %s activated successfully", user.Name),
				"user":    user,
			})
		}).
		Member("deactivate", "POST", "/deactivate", func(c *gin.Context) {
			userService := supergin.Resolve[UserService]("userService")
			idStr := c.Param("id")
			id, _ := strconv.Atoi(idStr)

			// Simulate deactivation
			user, err := userService.GetUser(id)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("User %s deactivated successfully", user.Name),
				"user":    user,
			})
		}).
		// Add custom collection routes
		Collection("stats", "GET", "/stats", func(c *gin.Context) {
			userService := supergin.Resolve[UserService]("userService")

			users, err := userService.ListUsers()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Calculate stats
			totalUsers := len(users)
			avgAge := 0
			if totalUsers > 0 {
				totalAge := 0
				for _, user := range users {
					totalAge += user.Age
				}
				avgAge = totalAge / totalUsers
			}

			c.JSON(http.StatusOK, gin.H{
				"total_users":  totalUsers,
				"average_age":  avgAge,
				"generated_at": time.Now(),
			})
		}).
		Build()

	// Add some additional demo routes
	app.Named("health_check").
		GET("/health").
		WithDescription("Health check endpoint").
		WithTags("health", "monitoring").
		Handler(func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"timestamp": time.Now(),
				"version":   "1.0.0",
			})
		})

	// DI testing route
	app.Named("di_test").
		GET("/di/test").
		WithDescription("Test dependency injection").
		WithTags("di", "test").
		Handler(func(c *gin.Context) {
			// Test different DI scopes
			db1 := supergin.Resolve[Database]("database")
			db2 := supergin.Resolve[Database]("database")

			userService1 := supergin.Resolve[UserService]("userService")
			userService2 := supergin.Resolve[UserService]("userService")

			c.JSON(http.StatusOK, gin.H{
				"message": "DI Test Results",
				"singleton_test": gin.H{
					"db1_equals_db2": fmt.Sprintf("%p", db1) == fmt.Sprintf("%p", db2),
					"explanation":    "Database should be the same instance (singleton)",
				},
				"request_scoped_test": gin.H{
					"userService1_equals_userService2": fmt.Sprintf("%p", userService1) == fmt.Sprintf("%p", userService2),
					"explanation":                      "UserService should be the same instance within this request",
				},
			})
		})

	// Print route summary
	fmt.Println("\n📋 Generated Routes:")
	fmt.Printf("   GET    /users           -> %s (List users)\n", userRoutes.List)
	fmt.Printf("   POST   /users           -> %s (Create user)\n", userRoutes.Create)
	fmt.Printf("   GET    /users/:id       -> %s (Get user)\n", userRoutes.Read)
	fmt.Printf("   PUT    /users/:id       -> %s (Update user)\n", userRoutes.Update)
	fmt.Printf("   DELETE /users/:id       -> %s (Delete user)\n", userRoutes.Delete)
	fmt.Printf("   GET    /users/search    -> %s (Search users)\n", userRoutes.Search)
	fmt.Println("   POST   /users/:id/activate   -> user_activate")
	fmt.Println("   POST   /users/:id/deactivate -> user_deactivate")
	fmt.Println("   GET    /users/stats          -> users_stats")
	fmt.Println("   GET    /health               -> health_check")
	fmt.Println("   GET    /di/test              -> di_test")
	fmt.Println("   GET    /api/docs             -> API documentation")

	fmt.Printf("\n🔗 Example URLs:\n")
	if listUrl, err := app.URLFor(userRoutes.List); err == nil {
		fmt.Printf("   List users: http://localhost:8080%s\n", listUrl)
	}
	if showUrl, err := app.URLFor(userRoutes.Read, "id", "1"); err == nil {
		fmt.Printf("   Get user:   http://localhost:8080%s\n", showUrl)
	}
}
