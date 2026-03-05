# GoNest Dependency Injection Module

Advanced dependency injection system with multiple scopes, provider types, and automatic injection.

## Features

- ✅ **Multiple Scopes**: Singleton, Transient, Request
- ✅ **Provider Types**: Class, Value, Factory, Async
- ✅ **Automatic Injection**: Field injection, method injection, auto-wiring
- ✅ **Hierarchical Containers**: Parent-child relationships
- ✅ **Type-Safe**: Full compile-time type checking
- ✅ **Thread-Safe**: Concurrent access support

## Scopes

### Singleton Scope

Single instance shared across the entire application.

```go
container.RegisterFactory(func() *DatabaseService {
  return &DatabaseService{dsn: "..."}
}, di.Singleton())
```

**Use cases:**
- Database connections
- Configuration services
- Caches
- Loggers

### Transient Scope

New instance created every time it's requested.

```go
container.RegisterFactory(func() *RequestHandler {
  return &RequestHandler{}
}, di.Transient())
```

**Use cases:**
- Request handlers
- Short-lived operations
- Stateful services that shouldn't be shared

### Request Scope

Single instance per HTTP request.

```go
container.RegisterFactory(func() *UserContext {
  return &UserContext{}
}, di.RequestScope())
```

**Use cases:**
- User context
- Request-specific data
- Per-request caches

## Provider Types

### Class Provider

Provides instances by calling a constructor function with dependency injection.

```go
// Constructor with dependencies
func NewUserService(db *DatabaseService) *UserService {
  return &UserService{db: db}
}

// Register
provider, _ := di.NewClassProvider(NewUserService, di.Singleton())
container.Register(provider, "")
```

**Auto dependency resolution:**
- Function parameters are resolved from container
- Supports `context.Context` as first parameter
- Can return `(T, error)` for error handling

### Value Provider

Provides a pre-existing instance.

```go
config := &AppConfig{Port: 3000}
container.RegisterValue(config, "")
```

**Use cases:**
- Configuration objects
- Pre-initialized services
- Constants

### Factory Provider

Provides instances using a custom factory function.

```go
factory := func(ctx context.Context, container *di.Container) (*CacheService, error) {
  config, _ := container.Resolve(ctx, reflect.TypeOf(&AppConfig{}))
  return NewCacheService(config.(*AppConfig)), nil
}

container.RegisterFactory(factory, di.Singleton())
```

**Factory signature:**
```go
func(ctx context.Context, container *di.Container, ...deps) (T, error)
```

### Async Provider

Provides instances asynchronously (e.g., from async initialization).

```go
asyncFactory := func(ctx context.Context) (*AsyncService, error) {
  service := &AsyncService{}
  if err := service.Initialize(ctx); err != nil {
    return nil, err
  }
  return service, nil
}

container.RegisterAsync(asyncFactory, di.Singleton())
```

## Container Usage

### Basic Registration

```go
container := di.NewContainer()

// Register value
config := &Config{Port: 3000}
container.RegisterValue(config, "")

// Register factory
container.RegisterFactory(func(cfg *Config) *DatabaseService {
  return NewDatabaseService(cfg)
}, di.Singleton())

// Register type
container.RegisterType(&UserService{}, di.Singleton())
```

### Resolution

```go
ctx := context.Background()

// Resolve by type
dbType := reflect.TypeOf((*DatabaseService)(nil))
db, err := container.Resolve(ctx, dbType)

// Resolve named provider
cache, err := container.ResolveNamed(ctx, cacheType, "redis")
```

### Named Providers

Multiple providers of the same type:

```go
// Register multiple caches
container.RegisterValue(redisCache, "redis")
container.RegisterValue(memCache, "memory")

// Resolve specific one
redis, _ := container.ResolveNamed(ctx, cacheType, "redis")
mem, _ := container.ResolveNamed(ctx, cacheType, "memory")
```

## Automatic Injection

### Field Injection

Use `inject` tag for automatic field injection:

```go
type UserController struct {
  UserService *UserService `inject:""`
  Cache     *CacheService `inject:"redis"` // Named injection
  Logger    *Logger     `inject:""`
}

controller := &UserController{}
injector := di.NewInjector(container)
injector.Inject(ctx, controller)
```

### Method Injection

Call methods with automatic parameter resolution:

```go
type Service struct{}

func (s *Service) Initialize(ctx context.Context, db *DatabaseService, cache *CacheService) error {
  // ...
}

results, err := injector.InjectMethod(ctx, service, "Initialize")
```

### Function Injection

Call any function with dependency injection:

```go
fn := func(db *DatabaseService, cache *CacheService) error {
  // ...
  return nil
}

results, err := injector.Call(ctx, fn)
```

### Auto-Wiring

Automatically create and wire an instance:

```go
instance, err := injector.AutoWire(ctx, &UserService{})
// Returns *UserService with all dependencies injected
```

## Scope Management

### Request Scope

```go
scopeManager := di.NewScopeManager(globalContainer)

// Create request scope
requestContainer, ctx := scopeManager.CreateRequestScope(ctx)

// Use request-scoped container
userCtx, _ := requestContainer.Resolve(ctx, userContextType)

// Cleanup after request
defer scopeManager.CleanupRequestScope(ctx)
```

### Hierarchical Containers

```go
// Global container
global := di.NewContainer()
global.RegisterValue(config, "")

// Module-specific container
moduleContainer := di.NewChildContainer(global)
moduleContainer.RegisterFactory(moduleServiceFactory, di.Singleton())

// Resolves from module first, then falls back to global
service, _ := moduleContainer.Resolve(ctx, serviceType)
```

## Advanced Examples

### Complete Service with DI

```go
// Define interfaces
type IUserRepository interface {
  FindByID(id string) (*User, error)
}

type IUserService interface {
  GetUser(id string) (*User, error)
}

// Implementations
type UserRepository struct {
  db *DatabaseService `inject:""`
}

type UserService struct {
  repo IUserRepository `inject:""`
}

// Setup container
container := di.NewContainer()

// Register dependencies
container.RegisterFactory(func() *DatabaseService {
  return &DatabaseService{dsn: "postgres://..."}
}, di.Singleton())

container.RegisterFactory(func(db *DatabaseService) IUserRepository {
  return &UserRepository{db: db}
}, di.Singleton())

container.RegisterFactory(func(repo IUserRepository) IUserService {
  return &UserService{repo: repo}
}, di.Singleton())

// Resolve and use
ctx := context.Background()
serviceType := reflect.TypeOf((*IUserService)(nil)).Elem()
service, _ := container.Resolve(ctx, serviceType)
userService := service.(IUserService)
```

### Circular Dependency Prevention

```go
// BAD: Circular dependency
type ServiceA struct {
  b *ServiceB `inject:""`
}

type ServiceB struct {
  a *ServiceA `inject:""`
}

// GOOD: Use interfaces
type IServiceA interface {
  DoA()
}

type IServiceB interface {
  DoB()
}

type ServiceA struct {
  b IServiceB `inject:""`
}

type ServiceB struct {
  a IServiceA `inject:""`
}
```

### Request-Scoped Services in HTTP Handler

```go
func UserHandler(w http.ResponseWriter, r *http.Request) {
  ctx := r.Context()
  
  // Create request scope
  requestContainer, ctx := scopeManager.CreateRequestScope(ctx)
  defer scopeManager.CleanupRequestScope(ctx)
  
  // Resolve request-scoped user context
  userCtxType := reflect.TypeOf((*UserContext)(nil))
  userCtx, _ := requestContainer.Resolve(ctx, userCtxType)
  
  // Use service
  // ...
}
```

## Best Practices

### 1. Prefer Interfaces Over Concrete Types

```go
// Good
type IUserService interface {
  GetUser(id string) (*User, error)
}

// Register interface
container.RegisterFactory(func() IUserService {
  return &UserService{}
}, di.Singleton())
```

### 2. Use Singleton for Stateless Services

```go
// Stateless services should be singleton
container.RegisterFactory(NewUserService, di.Singleton())
```

### 3. Use Transient for Stateful Operations

```go
// Stateful handlers should be transient
container.RegisterFactory(NewRequestHandler, di.Transient())
```

### 4. Avoid Circular Dependencies

- Use interfaces to break circular dependencies
- Lazy initialization when needed
- Restructure your architecture if circular deps are common

### 5. Named Providers for Multiple Implementations

```go
// Different cache implementations
container.RegisterValue(redisCache, "redis")
container.RegisterValue(memoryCache, "memory")
```

## Integration with Core Module

```go
// In module configuration
type UserModule struct{}

func (m *UserModule) Configure(b *core.ModuleBuilder) {
  // Setup DI container
  container := di.NewContainer()
  
  // Register providers
  container.RegisterFactory(NewUserService, di.Singleton())
  container.RegisterFactory(NewUserRepository, di.Singleton())
  
  // Create controller with injection
  injector := di.NewInjector(container)
  controller, _ := injector.AutoWire(ctx, &UserController{})
  
  b.Controllers(controller).Providers(container)
}
```

## Performance Considerations

- **Singleton**: O(1) after first resolution (cached)
- **Request**: O(1) per request (cached per request)
- **Transient**: O(n) always creates new instance
- **Reflection**: Used only during resolution, not runtime
- **Thread-Safe**: Uses RWMutex for concurrent access

## Testing

### Mock Dependencies

```go
// Create test container
testContainer := di.NewContainer()

// Register mock
mockDB := &MockDatabaseService{}
testContainer.RegisterValue(mockDB, "")

// Test with mocked dependencies
service := &UserService{}
injector := di.NewInjector(testContainer)
injector.Inject(ctx, service)

// service.db is now mockDB
```

## Common Patterns

### Factory with Multiple Dependencies

```go
container.RegisterFactory(func(
  db *DatabaseService,
  cache *CacheService,
  logger *Logger,
) *UserService {
  return &UserService{
    db:   db,
    cache:  cache,
    logger: logger,
  }
}, di.Singleton())
```

### Lazy Initialization

```go
type LazyService struct {
  container *di.Container
  instance  *HeavyService
  once    sync.Once
}

func (s *LazyService) GetService(ctx context.Context) *HeavyService {
  s.once.Do(func() {
    s.instance, _ = s.container.Resolve(ctx, heavyServiceType)
  })
  return s.instance
}
```

### Conditional Registration

```go
if config.UseRedis {
  container.RegisterValue(redisCache, "cache")
} else {
  container.RegisterValue(memoryCache, "cache")
}
```

## License

MIT