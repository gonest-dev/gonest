package main

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/gonest-dev/gonest/di"
)

// ========================================
// Interfaces
// ========================================

type ILogger interface {
	Info(msg string)
	Error(msg string)
}

type IDatabaseService interface {
	Connect() error
	Query(sql string) ([]map[string]any, error)
}

type IUserRepository interface {
	FindByID(id string) (*User, error)
	Save(user *User) error
}

type IUserService interface {
	GetUser(id string) (*User, error)
	CreateUser(name, email string) (*User, error)
}

// ========================================
// Models
// ========================================

type User struct {
	ID    string
	Name  string
	Email string
}

// ========================================
// Implementations
// ========================================

// Logger - Singleton
type Logger struct{}

var _ ILogger = (*Logger)(nil)

func NewLogger() ILogger {
	return &Logger{}
}

func (l *Logger) Info(msg string) {
	log.Printf("[INFO] %s", msg)
}

func (l *Logger) Error(msg string) {
	log.Printf("[ERROR] %s", msg)
}

// DatabaseService - Singleton
type DatabaseService struct {
	dsn    string
	logger ILogger
}

var _ IDatabaseService = (*DatabaseService)(nil)

func NewDatabaseService(logger ILogger) (IDatabaseService, error) {
	db := &DatabaseService{
		dsn:    "postgres://localhost:5432/mydb",
		logger: logger,
	}

	if err := db.Connect(); err != nil {
		return nil, err
	}

	return db, nil
}

func (d *DatabaseService) Connect() error {
	d.logger.Info(fmt.Sprintf("Connecting to database: %s", d.dsn))
	return nil
}

func (d *DatabaseService) Query(sql string) ([]map[string]any, error) {
	d.logger.Info(fmt.Sprintf("Executing query: %s", sql))
	return []map[string]any{}, nil
}

// UserRepository - Singleton
type UserRepository struct {
	db     IDatabaseService
	logger ILogger
}

var _ IUserRepository = (*UserRepository)(nil)

func NewUserRepository(db IDatabaseService, logger ILogger) IUserRepository {
	return &UserRepository{
		db:     db,
		logger: logger,
	}
}

func (r *UserRepository) FindByID(id string) (*User, error) {
	r.logger.Info(fmt.Sprintf("Finding user by ID: %s", id))

	// Simulate DB query
	return &User{
		ID:    id,
		Name:  "John Doe",
		Email: "john@example.com",
	}, nil
}

func (r *UserRepository) Save(user *User) error {
	r.logger.Info(fmt.Sprintf("Saving user: %s", user.Name))
	return nil
}

// UserService - Singleton
type UserService struct {
	repo   IUserRepository
	logger ILogger
}

var _ IUserService = (*UserService)(nil)

func NewUserService(repo IUserRepository, logger ILogger) IUserService {
	return &UserService{
		repo:   repo,
		logger: logger,
	}
}

func (s *UserService) GetUser(id string) (*User, error) {
	s.logger.Info(fmt.Sprintf("Getting user: %s", id))
	return s.repo.FindByID(id)
}

func (s *UserService) CreateUser(name, email string) (*User, error) {
	s.logger.Info(fmt.Sprintf("Creating user: %s", name))

	user := &User{
		ID:    "generated-id",
		Name:  name,
		Email: email,
	}

	if err := s.repo.Save(user); err != nil {
		return nil, err
	}

	return user, nil
}

// RequestContext - Request Scoped
type RequestContext struct {
	RequestID string
	UserID    string
}

func NewRequestContext() *RequestContext {
	return &RequestContext{
		RequestID: "req-123",
	}
}

// TransientHandler - Transient (new instance per request)
type TransientHandler struct {
	handlerID int
}

var handlerCounter int

func NewTransientHandler() *TransientHandler {
	handlerCounter++
	return &TransientHandler{
		handlerID: handlerCounter,
	}
}

// ========================================
// Main
// ========================================

func main() {
	ctx := context.Background()

	// Create container
	container := di.NewContainer()

	fmt.Println("========================================")
	fmt.Println("GoNest DI Example")
	fmt.Println("========================================")
	fmt.Println()

	// ========================================
	// 1. Singleton Providers
	// ========================================
	fmt.Println("1. Registering Singleton Providers...")

	// Logger - Singleton
	container.RegisterFactory(NewLogger, di.Singleton())
	fmt.Println("   ✓ Logger registered (Singleton)")

	// DatabaseService - Singleton with dependencies
	container.RegisterFactory(NewDatabaseService, di.Singleton())
	fmt.Println("   ✓ DatabaseService registered (Singleton)")

	// UserRepository - Singleton
	container.RegisterFactory(NewUserRepository, di.Singleton())
	fmt.Println("   ✓ UserRepository registered (Singleton)")

	// UserService - Singleton
	container.RegisterFactory(NewUserService, di.Singleton())
	fmt.Println("   ✓ UserService registered (Singleton)")
	fmt.Println()

	// ========================================
	// 2. Request Scoped Provider
	// ========================================
	fmt.Println("2. Registering Request Scoped Provider...")
	container.RegisterFactory(NewRequestContext, di.Request())
	fmt.Println("   ✓ RequestContext registered (Request Scope)")
	fmt.Println()

	// ========================================
	// 3. Transient Provider
	// ========================================
	fmt.Println("3. Registering Transient Provider...")
	container.RegisterFactory(NewTransientHandler, di.Transient())
	fmt.Println("   ✓ TransientHandler registered (Transient)")

	// ========================================
	// 4. Resolve Singleton
	// ========================================
	fmt.Println("4. Resolving Singleton Services...")

	userServiceType := reflect.TypeOf((*IUserService)(nil)).Elem()
	service1, _ := container.Resolve(ctx, userServiceType)
	userService1 := service1.(IUserService)
	fmt.Println("   ✓ First UserService instance resolved")

	service2, _ := container.Resolve(ctx, userServiceType)
	userService2 := service2.(IUserService)
	fmt.Println("   ✓ Second UserService instance resolved")

	if userService1 == userService2 {
		fmt.Println("   ✓ Both instances are the SAME (Singleton works!)")
	}

	// ========================================
	// 5. Use Service
	// ========================================
	fmt.Println("\n5. Using UserService...")
	user, _ := userService1.GetUser("123")
	fmt.Printf("   User found: %s (%s)\n\n", user.Name, user.Email)

	newUser, _ := userService1.CreateUser("Jane Doe", "jane@example.com")
	fmt.Printf("   User created: %s (%s)\n\n", newUser.Name, newUser.Email)

	// ========================================
	// 6. Request Scoped Resolution
	// ========================================
	fmt.Println("6. Testing Request Scoped Services...")

	reqCtxType := reflect.TypeOf((*RequestContext)(nil))

	// First request scope
	ctx1, _ := container.Resolve(ctx, reqCtxType)
	reqCtx1 := ctx1.(*RequestContext)
	reqCtx1.UserID = "user-1"
	fmt.Printf("   Request 1 - UserID: %s\n", reqCtx1.UserID)

	ctx1Again, _ := container.Resolve(ctx, reqCtxType)
	reqCtx1Again := ctx1Again.(*RequestContext)
	fmt.Printf("   Request 1 (again) - UserID: %s\n", reqCtx1Again.UserID)

	if reqCtx1 == reqCtx1Again {
		fmt.Println("   ✓ Same instance within request scope!")
	}

	// Clear request scope and resolve again
	container.ClearRequestScope()
	ctx2, _ := container.Resolve(ctx, reqCtxType)
	reqCtx2 := ctx2.(*RequestContext)
	reqCtx2.UserID = "user-2"
	fmt.Printf("\n   Request 2 - UserID: %s\n", reqCtx2.UserID)

	if reqCtx1 != reqCtx2 {
		fmt.Println("   ✓ Different instance in new request scope!")
	}

	// ========================================
	// 7. Transient Resolution
	// ========================================
	fmt.Println()
	fmt.Println("7. Testing Transient Services...")

	handlerType := reflect.TypeOf((*TransientHandler)(nil))

	h1, _ := container.Resolve(ctx, handlerType)
	handler1 := h1.(*TransientHandler)
	fmt.Printf("   Handler 1 - ID: %d\n", handler1.handlerID)

	h2, _ := container.Resolve(ctx, handlerType)
	handler2 := h2.(*TransientHandler)
	fmt.Printf("   Handler 2 - ID: %d\n", handler2.handlerID)

	if handler1 != handler2 {
		fmt.Println("   ✓ Different instances every time (Transient works!)")
	}

	// ========================================
	// 8. Automatic Injection
	// ========================================
	fmt.Println()
	fmt.Println("8. Testing Automatic Injection...")

	type Controller struct {
		UserService IUserService    `inject:""`
		Logger      ILogger         `inject:""`
		ReqContext  *RequestContext `inject:""`
	}

	controller := &Controller{}
	injector := di.NewInjector(container)
	injector.Inject(ctx, controller)

	fmt.Println("   ✓ Dependencies injected automatically")
	fmt.Printf("   UserService injected: %v\n", controller.UserService != nil)
	fmt.Printf("   Logger injected: %v\n", controller.Logger != nil)
	fmt.Printf("   RequestContext injected: %v\n\n", controller.ReqContext != nil)

	// Use injected dependencies
	controller.Logger.Info("Controller initialized with DI!")
	user, _ = controller.UserService.GetUser("456")
	fmt.Printf("   User via controller: %s\n\n", user.Name)

	// ========================================
	// Summary
	// ========================================
	fmt.Println("========================================")
	fmt.Println("Summary:")
	fmt.Println("✓ Singleton: Same instance shared")
	fmt.Println("✓ Request Scope: Same per request")
	fmt.Println("✓ Transient: New instance every time")
	fmt.Println("✓ Automatic Injection: Working!")
	fmt.Println("✓ Dependency Resolution: Working!")
	fmt.Println("========================================")
}
