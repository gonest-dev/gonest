# GoNest Framework Roadmap

> A NestJS-inspired framework for Go with advanced DI, decorators, and type-safe validation

## 🎯 Current Status

**Phase 1: Foundation** ✅ **COMPLETED** (March 2026)
- Core Architecture: 100% complete
- Advanced DI: 100% complete (all scopes + providers)
- Context System: 100% complete
- Lifecycle Hooks: 100% complete
- Example Applications: 2 complete

**Phase 2: Type-Safe Validation** ✅ **COMPLETED** (March 2026)
- Validator Core: 100% complete
- Built-in Rules: 86+ rules across 9 categories
- Advanced Validation: 100% complete
- Example Applications: 1 complete

**Phase 3: Decorators & Routing** ✅ **COMPLETED** (March 2026)
- Controller System: 100% complete
- HTTP Method Decorators: 7 methods complete
- Parameter Extraction: 100% complete
- ValidationPipe: 100% complete
- Parse Pipes: 8 pipes complete
- Example Applications: 1 complete

**Phase 4: Guards & Security** ✅ **COMPLETED** (March 2026)
- Guard System: 100% complete
- Built-in Guards: 3 guards complete
- Security Features: 100% complete
- Example Applications: 1 complete

**Phase 5: Interceptors & Middleware** ✅ **COMPLETED** (March 2026)
- Interceptor System: 100% complete
- Built-in Interceptors: 5 interceptors complete
- Per-route & Controller-level: 100% complete
- Example Applications: 1 complete

**Phase 6: Exception Handling** ✅ **COMPLETED** (March 2026)
- Exception System: 100% complete
- HTTP Exceptions: 8 types complete
- Exception Filters: 5 filters complete
- Example Applications: 1 complete

**Phase 7: Swagger/OpenAPI** ✅ **COMPLETED** (March 2026)
- Swagger Core: 100% complete
- Descriptor API: 100% complete (type-safe, no tags!)
- OpenAPI 3.0.3: 100% complete
- Validator enhanced with callback API
- Example Applications: 1 complete (+ schema-simple)

**Next Up: Phase 8 - Platform Adapters** 🚧

---

## Vision

Create a production-ready, type-safe, and developer-friendly framework for building scalable server-side applications in Go, inspired by NestJS but leveraging Go's strengths.

---

## Phase 1: Foundation (Months 1-2) 🏗️ ✅ COMPLETED

### Core Architecture
- [x] Project structure setup
- [x] Basic package organization
- [x] Core module system
  - [x] Module interface and builder
  - [x] Module metadata storage
  - [x] Module dependency resolution
  - [x] Circular dependency detection
- [x] Application lifecycle
  - [x] OnModuleInit hook
  - [x] OnModuleDestroy hook
  - [x] OnApplicationBootstrap hook
  - [x] OnApplicationShutdown hook
  - [x] Graceful shutdown handling

### Dependency Injection
- [x] DI Container implementation ✅
  - [x] Provider registration
  - [x] Dependency resolution (automatic)
  - [x] Manual injection via module configuration
  - [x] Automatic injection via reflection
- [x] Scopes system ✅
  - [x] Singleton scope
  - [x] Transient scope
  - [x] Request scope
  - [x] Scope manager
- [x] Provider types ✅
  - [x] Class providers
  - [x] Value providers
  - [x] Factory providers
  - [x] Async providers
- [x] Circular dependency handling ✅
  - [x] Detection via reflection
  - [x] Prevention through interfaces
- [x] Advanced features ✅
  - [x] Field injection (`inject` tag)
  - [x] Method injection
  - [x] Function injection
  - [x] Auto-wiring
  - [x] Named providers
  - [x] Hierarchical containers

### Context System
- [x] Request context implementation
- [x] Context methods (JSON, Status, BindJSON, etc)
- [x] Context middleware chain
- [x] Context metadata storage
- [x] Context cancellation support

---

## Phase 2: Type-Safe Validation (Month 2) ✅ **COMPLETED**

### Validator Core
- [x] Core validator types and interfaces
- [x] Field validator builder
- [x] Schema builder with generics
- [x] Validation result structure
- [x] Error handling and formatting

### Built-in Rules (86+ total)
- [x] **Common rules** (9 rules)
  - [x] Required(), Optional(), NotEmpty()
  - [x] Custom(), Must()
  - [x] Equal(), NotEqual(), OneOf(), In()
- [x] **String rules** (17 rules)
  - [x] Email(), URL(), UUID()
  - [x] MinLength(n), MaxLength(n), Length(n)
  - [x] Pattern(regex), AlphaNumeric(), Alpha(), Numeric()
  - [x] Contains(), StartsWith(), EndsWith()
  - [x] HasUpperCase(), HasLowerCase(), HasDigit(), HasSpecialChar()
  - [x] StrongPassword()
- [x] **Number rules** (12 rules)
  - [x] Min(n), Max(n), Range(min, max), Between()
  - [x] Positive(), Negative(), NonNegative(), NonPositive()
  - [x] GreaterThan(), LessThan(), GreaterThanOrEqual(), LessThanOrEqual()
  - [x] MultipleOf()
- [x] **Boolean rules** (4 rules)
  - [x] IsTrue(), IsFalse()
  - [x] MustAccept(), MustDecline()
- [x] **Comparison rules** (11 rules)
  - [x] EqualTo(), NotEqualTo(), SameAs(), DifferentFrom()
  - [x] InRange(), NotInRange(), NotIn()
  - [x] Compare()
  - [x] When(), Unless() (conditional validation)
- [x] **Date rules** (11 rules)
  - [x] DateAfter(date), DateBefore(date), DateBetween(start, end)
  - [x] DatePast(), DateFuture(), DateToday()
  - [x] DateMinAge(), DateMaxAge()
  - [x] DateWeekday(), DateWeekend(), DateIsWeekday()
- [x] **Array rules** (11 rules)
  - [x] ArrayMinSize(n), ArrayMaxSize(n), ArraySize(n)
  - [x] ArrayNotEmpty(), ArrayUnique()
  - [x] ArrayContains(value), ArrayDoesNotContain()
  - [x] ArrayEvery(), ArraySome(), ArrayNone()
  - [x] ArrayEach() (validates each element)
- [x] **Async rules** (5 helpers)
  - [x] AsyncCustom()
  - [x] AsyncUnique() (database uniqueness check)
  - [x] AsyncExists() (foreign key validation)
  - [x] AsyncValidateWith()
  - [x] AsyncCompare()
- [x] **Struct rules** (6 rules)
  - [x] ValidStruct(), ValidStructPtr()
  - [x] ValidStructAsync(), ValidStructPtrAsync()
  - [x] StructField(), StructHas()

### Advanced Validation
- [x] Cross-field validation
- [x] Conditional validation (When/Unless)
- [x] Async validation support
- [x] Custom validator registration
- [x] Nested object validation
- [x] Array item validation
- [x] Detailed error messages with codes and params
- [x] JSON error formatting

### Integration
- [x] DTO validation examples
- [x] Error response formatting
- [x] HTTP controller integration examples

### Examples & Documentation
- [x] examples/validation/main.go (8 examples)
- [x] examples/validation-advanced/main.go (7 advanced examples)
- [x] validator/README.md (comprehensive guide)
- [x] Performance best practices documented

---

## Phase 3: Decorators & Routing (Month 3) ✅ **COMPLETED**

### Controller System
- [x] Controller interface
- [x] Controller builder pattern
- [x] Route definition structure
- [x] Controller metadata extraction
- [x] Auto-registration system

### Route Decorators
- [x] HTTP method decorators
  - [x] Get(path)
  - [x] Post(path)
  - [x] Put(path)
  - [x] Patch(path)
  - [x] Delete(path)
  - [x] Options(path)
  - [x] Head(path)
- [x] Route configuration
  - [x] Path parameters
  - [x] Query parameters
  - [x] Headers
  - [x] Body binding

### Parameter Decorators
- [x] @Body() - Request body with validation
- [x] @Query() - Query parameters
- [x] @Param() - Path parameters
- [x] @Headers() - Request headers
- [x] @Req() - Raw request (Context)
- [x] @Res() - Raw response (Context)

### Validation Integration (from Phase 2)
- [x] ValidationPipe implementation
- [x] Auto-validation decorator (ValidateBody)
- [x] Automatic DTO validation on routes
- [x] Validation error transformation

### Parse Pipes
- [x] ParseIntPipe
- [x] ParseFloatPipe
- [x] ParseBoolPipe
- [x] ParseUUIDPipe
- [x] ParseEnumPipe
- [x] ParseArrayPipe
- [x] DefaultValuePipe

### Examples & Documentation
- [x] examples/controller-basic/main.go
- [x] examples/pipes-validation/main.go
- [x] controller/README.md
- [x] pipes/README.md

**Total: Controller System + Pipes System 100% Complete!**

### Parse Pipes
- [ ] ParseIntPipe
- [ ] ParseFloatPipe
- [ ] ParseBoolPipe
- [ ] ParseUUIDPipe
- [ ] ParseDatePipe
- [ ] ParseEnumPipe
- [ ] ParseArrayPipe
- [ ] DefaultValuePipe

---

## Phase 4: Guards & Security (Month 4) ✅ **COMPLETED**

### Guard System
- [x] Guard interface
- [x] Guard execution context
- [x] Guard chaining
- [x] Global guards
- [x] Route-level guards

### Built-in Guards
- [x] AuthGuard (JWT, Bearer)
- [x] RolesGuard
- [x] ThrottlerGuard (rate limiting)

### Security Features
- [x] Token validation
- [x] Role-based access control (RBAC)
- [x] Rate limiting with in-memory store
- [x] Custom guard errors
- [x] Structured error responses

### Examples & Documentation
- [x] examples/guards-security/main.go
- [x] guards/README.md

**Total: Guards & Security System 100% Complete!**

---

## Phase 5: Interceptors & Middleware (Month 5) ✅ **COMPLETED**

### Interceptor System
- [x] Interceptor interface
- [x] Execution context
- [x] Before/After handling
- [x] Global interceptors
- [x] Route-level interceptors

### Built-in Interceptors
- [x] LoggingInterceptor (request/response logging)
- [x] TimeoutInterceptor (request timeout)
- [x] CacheInterceptor (response caching)
- [x] TransformInterceptor (response transformation)
- [x] ErrorInterceptor (error handling)

### Features
- [x] Composable interceptors
- [x] Controller-level application
- [x] Route-level application
- [x] Execution order control
- [x] Context metadata
- [x] Async support

### Examples & Documentation
- [x] examples/interceptors/main.go
- [x] Per-route examples
- [x] Hybrid (controller + route) examples
- [x] interceptors/README.md

**Total: Interceptors & Middleware System 100% Complete!**

---

## Phase 6: Exception Handling (Month 6) ✅ **COMPLETED**

### Exception Filters
- [x] Exception filter interface
- [x] Global exception filters
- [x] Route-level filters
- [x] Custom exception filters
- [x] Filter chaining

### HTTP Exceptions
- [x] BadRequestException (400)
- [x] UnauthorizedException (401)
- [x] ForbiddenException (403)
- [x] NotFoundException (404)
- [x] ConflictException (409)
- [x] UnprocessableEntityException (422)
- [x] InternalServerErrorException (500)
- [x] ServiceUnavailableException (503)
- [x] Custom exception creation

### Error Handling
- [x] Structured error responses
- [x] Exception details & metadata
- [x] Error logging integration
- [x] ValidationException integration

### Built-in Filters
- [x] GlobalExceptionFilter
- [x] NotFoundExceptionFilter
- [x] ValidationExceptionFilter
- [x] UnauthorizedExceptionFilter
- [x] ForbiddenExceptionFilter

### Examples & Documentation
- [x] examples/exceptions/main.go
- [x] 10 exception type examples
- [x] Custom filter examples
- [x] exceptions/README.md

**Total: Exception Handling System 100% Complete!**

---

## Phase 7: Swagger/OpenAPI Integration (Month 7) ✅ **COMPLETED**

### Swagger Core
- [x] OpenAPI 3.0.3 document builder
- [x] Type-safe Descriptor API (no struct tags!)
- [x] Pointer-based field selection
- [x] Schema generator from types
- [x] Swagger UI integration
- [x] JSON export

### Descriptor API
- [x] Clean callback syntax
- [x] Type-safe field references
- [x] Compile-time checking
- [x] IDE autocomplete support
- [x] Consistent with validator module

### OpenAPI Features
- [x] Info, Contact, License
- [x] Multiple servers
- [x] Tags organization
- [x] Security schemes (Bearer, API Key)
- [x] Request/Response schemas
- [x] Parameters (path, query, header)
- [x] Complete CRUD documentation

### Field Descriptors (15+ methods)
- [x] Description, Example, Format
- [x] Required, Optional
- [x] Minimum, Maximum (numbers)
- [x] MinLength, MaxLength (strings)
- [x] Pattern (regex)
- [x] Enum (allowed values)
- [x] WriteOnly, ReadOnly
- [x] Deprecated, Default

### Examples & Documentation
- [x] examples/swagger/main.go
- [x] Complete CRUD example
- [x] Multiple DTOs with descriptors
- [x] swagger/README.md

**Validator Module Updated:**
- [x] Schema callback API added
- [x] Consistent with Swagger Descriptor
- [x] Type SchemaType[T] (no naming conflict)
- [x] Clean callback syntax

**Total: Swagger/OpenAPI + Enhanced Validator 100% Complete!**

---

## Phase 8: Platform Adapters (Month 7) 🔌

### HTTP Adapters
- [ ] Gin adapter
  - [ ] Route registration
  - [ ] Middleware integration
  - [ ] Context adaptation
- [ ] Fiber adapter
  - [ ] High-performance implementation
  - [ ] WebSocket support
- [ ] Echo adapter
- [ ] net/http adapter (standard library)

### Platform Abstraction
- [ ] Router interface
- [ ] Request/Response abstraction
- [ ] Platform-agnostic middleware
- [ ] Adapter benchmarks
- [ ] Adapter selection guide

---

## Phase 9: Advanced Features (Month 8) 🚀

### Testing Utilities
- [ ] TestingModule
- [ ] Mock providers
- [ ] E2E testing helpers
- [ ] Request testing utilities
- [ ] Coverage tools

### Configuration Module
- [ ] Config service
- [ ] Environment variables
- [ ] .env file support
- [ ] Config validation
- [ ] Typed configuration
- [ ] Hot reload support

### Caching Module
- [ ] Cache interface
- [ ] In-memory cache
- [ ] Redis adapter
- [ ] Memcached adapter
- [ ] Cache decorators
- [ ] TTL management

### Task Scheduling
- [ ] Cron jobs
- [ ] Interval tasks
- [ ] Timeout tasks
- [ ] Task queue integration

### Events Module
- [ ] Event emitter
- [ ] Event listeners
- [ ] Async event handling
- [ ] Event namespacing

---

## Phase 10: Database & ORM Integration (Month 9) 💾

### Database Module
- [ ] Database connection management
- [ ] Connection pooling
- [ ] Transaction support
- [ ] Migration helpers

### ORM Integrations
- [ ] GORM integration
  - [ ] Repository pattern
  - [ ] Entity decorators
  - [ ] Automatic CRUD
- [ ] SQLx integration
- [ ] Ent integration
- [ ] Prisma Go client

### Repository Pattern
- [ ] Generic repository
- [ ] Custom repositories
- [ ] Query builders
- [ ] Specification pattern

---

## Phase 11: Microservices (Month 10) 🌐

### Transport Layers
- [ ] TCP transport
- [ ] Redis transport
- [ ] NATS transport
- [ ] RabbitMQ transport
- [ ] Kafka transport
- [ ] gRPC transport

### Microservice Patterns
- [ ] Request-response pattern
- [ ] Event-based communication
- [ ] Message queuing
- [ ] Service discovery
- [ ] Load balancing
- [ ] Circuit breaker

### gRPC Support
- [ ] Protocol buffer integration
- [ ] Streaming support
- [ ] Metadata handling
- [ ] Interceptors for gRPC

---

## Phase 12: GraphQL Support (Month 11) 📊

### GraphQL Core
- [ ] Schema-first approach
- [ ] Code-first approach
- [ ] Resolver decorators
- [ ] Query complexity analysis
- [ ] DataLoader integration

### Advanced GraphQL
- [ ] Subscriptions
- [ ] Federation support
- [ ] Custom scalars
- [ ] Directives
- [ ] Apollo integration

---

## Phase 13: WebSockets (Month 11) 🔌

### WebSocket Gateway
- [ ] Gateway decorators
- [ ] Event listeners
- [ ] Emit decorators
- [ ] Room management
- [ ] Namespace support

### Real-time Features
- [ ] Broadcasting
- [ ] Private channels
- [ ] Presence channels
- [ ] Socket.io compatibility

---

## Phase 14: CLI Tools (Month 12) 🛠️

### GoNest CLI
- [ ] Project scaffolding
- [ ] Module generation
- [ ] Controller generation
- [ ] Service generation
- [ ] Guard generation
- [ ] Interceptor generation
- [ ] Pipe generation
- [ ] Migration tools

### CLI Commands
```bash
gonest new <project-name>
gonest generate module <name>
gonest generate controller <name>
gonest generate service <name>
gonest generate guard <name>
gonest generate interceptor <name>
gonest generate pipe <name>
gonest info
gonest build
```

---

## Phase 15: Documentation & Examples (Ongoing) 📖

### Documentation
- [ ] Getting started guide
- [ ] Core concepts
- [ ] API reference
- [ ] Best practices
- [ ] Migration guides
- [ ] Video tutorials
- [ ] Interactive playground

### Example Applications
- [ ] REST API example
- [ ] GraphQL API example
- [ ] Microservices example
- [ ] WebSocket chat example
- [ ] CRUD with database
- [ ] Authentication & authorization
- [ ] File upload example
- [ ] Testing examples

### Recipes & Guides
- [ ] Authentication recipes
- [ ] Database patterns
- [ ] Testing strategies
- [ ] Deployment guides
- [ ] Performance optimization
- [ ] Security best practices

---

## Phase 16: Performance & Optimization (Month 13) ⚡

### Performance
- [ ] Benchmark suite
- [ ] Memory profiling
- [ ] CPU profiling
- [ ] Optimization strategies
- [ ] Caching strategies
- [ ] Connection pooling
- [ ] Lazy loading

### Monitoring
- [ ] Prometheus integration
- [ ] Metrics collection
- [ ] Health checks
- [ ] Readiness probes
- [ ] Liveness probes
- [ ] Distributed tracing (OpenTelemetry)

---

## Phase 17: DevOps & Production (Month 14) 🚢

### Docker Support
- [ ] Dockerfile templates
- [ ] Docker Compose examples
- [ ] Multi-stage builds
- [ ] Alpine/Distroless images

### Kubernetes
- [ ] Deployment manifests
- [ ] Service manifests
- [ ] ConfigMap examples
- [ ] Secret management
- [ ] Health checks

### CI/CD
- [ ] GitHub Actions workflows
- [ ] GitLab CI examples
- [ ] Testing pipelines
- [ ] Deployment pipelines

---

## Phase 18: Community & Ecosystem (Ongoing) 🌟

### Community Building
- [ ] Discord/Slack community
- [ ] Contributing guidelines
- [ ] Code of conduct
- [ ] Issue templates
- [ ] PR templates
- [ ] Roadmap voting

### Plugin System
- [ ] Plugin architecture
- [ ] Plugin registry
- [ ] Community plugins
- [ ] Plugin documentation

### Integrations
- [ ] Popular library integrations
- [ ] Third-party services
- [ ] Cloud provider SDKs
- [ ] Monitoring services

---

## Version Milestones

### v0.1.0 - Alpha ✅ **ACHIEVED** (Month 3 - March 2026)
- ✅ Core module system
- ✅ Advanced DI container (all scopes + providers)
- ✅ Complete routing
- ✅ Type-safe validation (86+ rules)
- ✅ Controllers & Pipes
- ✅ Guards & Security
- ✅ Multiple example applications

**Current Status:** v0.1.0 Complete! Moving to Beta features.

### v0.5.0 - Beta (Month 6 - Target: June 2026)
- [x] Complete validation system ✅ 
- [x] Guards ✅ (interceptors in progress)
- [ ] Swagger integration
- [ ] Platform adapters

### v1.0.0 - Stable (Month 12 - Target: December 2026)
- Production-ready core
- Complete documentation
- CLI tools
- Database integration
- Microservices support

### v2.0.0 - Advanced (Month 18 - Target: June 2027)
- GraphQL support
- Advanced microservices
- Enhanced performance
- Enterprise features

---

## Success Metrics

### Technical Goals
- [ ] 90%+ test coverage
- [ ] < 50ms average response time
- [ ] < 100MB memory footprint
- [ ] Zero breaking changes in minor versions

### Community Goals
- [ ] 1,000+ GitHub stars
- [ ] 100+ contributors
- [ ] 50+ production deployments
- [ ] Active community forum

### Documentation Goals
- [ ] 100% API documentation
- [ ] 20+ comprehensive guides
- [ ] 10+ video tutorials
- [ ] Interactive examples

---

## Contributing

This roadmap is a living document. Community feedback is essential for prioritizing features and improvements.

**How to contribute:**
1. Open issues for feature requests
2. Comment on existing issues with use cases
3. Vote on features you want to see
4. Submit PRs for roadmap items
5. Share your success stories

---

## Notes

- Priorities may shift based on community feedback
- Some features may be implemented in parallel
- Breaking changes will follow semantic versioning
- Enterprise features may be developed separately

---

**Last Updated:** March 2026
**Next Review:** April 2026

---

## Quick Links

- [GitHub Repository](https://github.com/leandroluk/gonest)
- [Documentation](https://gonest.dev/docs)
- [Discord Community](https://discord.gg/gonest)
- [Contributing Guide](./CONTRIBUTING.md)
- [License](./LICENSE)