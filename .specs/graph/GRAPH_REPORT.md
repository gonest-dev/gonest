# Graph Report - .  (2026-08-22)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 3416 nodes · 7486 edges · 248 communities (191 shown, 57 thin omitted)
- Extraction: 81% EXTRACTED · 19% INFERRED · 0% AMBIGUOUS · INFERRED: 1410 edges (avg confidence: 0.8)
- Token cost: 10,770 input · 2,761 output

## Graph Freshness
- Built from commit: `fe4b83f6`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Provider Lifecycle Testing
- Application Shutdown Sequence
- Route Schema Testing
- Dotenv Environment Loading
- HTTP Request Mocking
- Service Integration Testing
- Event Emitter System
- Logging Configuration
- Pipeline Dispatch Testing
- OpenAPI Schema Generation
- Request Validation Exceptions
- HTTP Server Adapters
- Dependency Injection Management
- Controller Declaration Testing
- Task Scheduler Module
- App Listener Testing
- Search Query Schema
- Module Assembly Logic
- Array Schema Validation
- Route Metadata Definition
- Module Internal References
- Property Builder Fluent API
- Parameter Coercion Logic
- Dirty-Tracking Accessors
- HTTP Middleware Pipeline
- Standard HTTP Exceptions
- Application Bootstrap Utilities
- WebSocket Protocol Testing
- HTTP Responder Mock
- Controller Metadata Definition
- Dependency Resolution Engine
- Middleware Handler Testing
- Bootstrap Order Tracking
- Fiber Web Framework Adapter
- HTTP Response Mocking
- Fiber Integration Testing
- Direct Provider Resolution
- Array Schema Definition
- Lifecycle Hook Invocation
- GraphQL Resolver Definition
- GraphQL Operation Registry
- Injection Ordering Tests
- String Schema Definition
- App Routing Pipeline
- Request Validation Helpers
- Service Provider Mocking
- Provider Reference Metadata
- Schema Property Registration
- Direct Dependency Lookup
- SSE Responder Mock
- Direct Dispatch Testing
- OpenAPI Document Builder
- Request Body Parsing
- Dependency Graph Traversal
- JSON Sync Logic
- Exception Name Mapping
- GraphQL Field Builder
- SSE Connection Handling
- Numeric Schema Definition
- Interface Provider Validation
- HTTP Exception Details
- Service Logging Spies
- GraphQL Schema Building
- Module Export Validation
- Provider Resolution Testing
- Numeric Constraint Testing
- Parameter Mocking Helpers
- Built-in Auth Exceptions
- Request Source Testing
- Dependency Injection Core
- Schema Field Reflection
- Form Data Mocking
- Route Parameter Validation
- Query String Mocking
- Fiber HTTP Responder
- Test App Harness
- HTTP Response Utilities
- Mock Service Entities
- Request Parameter Mocking
- Query Parameter Validation
- Duration Validation Testing
- Generic Request Mocking
- Circular Dependency Detection
- GraphQL Mutation Definition
- Provider Type Casting
- OpenAPI Specification Metadata
- Request Scoped Cache
- String Constraint Testing
- Swagger Documentation Setup
- Extended HTTP Exceptions
- GraphQL Execution Logic
- Accessor State Sync
- Distinct SSE Streams
- Provider Casting Tests
- Dependency Graph Visualization
- Value Schema Definition
- Data Transfer Objects
- Lifecycle Event Recording
- Entity CRUD Interfaces
- Service Layer Implementation
- Todo Service Logic
- GraphQL Request Handling
- Environment Variable Parsing
- Multipart Form Parsing
- HTTP Response Handling
- Duration Schema Definition
- Duration Validation Tests
- Schema Kind Validation
- App Lifecycle Hooks
- User Controller Integration
- OpenAPI Metadata Configuration
- Swagger UI Integration
- DateTime Schema Validation
- Object Schema Definition
- Schema Type Registry
- Enum Validation Tests
- GraphQL Argument Normalization
- Form File Uploads
- Schema Refinement Logic
- Custom Validation Functions
- Generic State Accessor
- Blog GraphQL Service
- User Service Exceptions
- WebSocket Connection Upgrade
- Lazy Dependency Injection
- GraphQL Protocol Testing
- HTTP Reply Tests
- GraphQL Resolver Declaration
- Lazy Module Loading
- JSON Response Assertions
- Fiber WebSocket Adapter
- Default Value Handling
- Lazy Module Integration
- Database Lifecycle Hooks
- User Provider Resolution
- Lazy Module Definition
- Dependency Injection Tokens
- HTTP Method Constants
- Custom Schema Properties
- Project Module List
- Event Emitter Subscription
- Post Service Logic
- GraphQL Scalar Mapping
- Lifecycle Phase Orchestration
- Constructor Injection Constraints
- Comment Service Logic
- Request Reservation Registry
- Module Export Resolution
- Post Data Transfer Objects
- Exception Schema Shape
- App Options Configuration
- Todo Controller DTOs
- Core Framework Types
- Cross-Transport Event Testing
- Direct Dependency Resolution
- Direct Dependency Resolution
- Blog GraphQL DTOs
- Comment DTOs
- Email Service
- User DTOs
- Database Connection
- SMS Service
- Failed Dependency Exception
- Gateway Timeout Exception
- Insufficient Storage Exception
- Internal Server Error
- Locked Resource Exception
- Loop Detected Exception
- Misdirected Request Exception
- Network Auth Exception
- Not Extended Exception
- Precondition Failed Exception
- Range Not Satisfiable
- Header Too Large
- Request Timeout Exception
- Service Unavailable Exception
- Teapot Exception
- Too Many Requests
- Unprocessable Entity Exception
- Upgrade Required Exception
- Variant Negotiates Exception
- OpenAPI Security Document
- Filter and Middleware
- Filter Design Specs
- GraphQL Realtime Specs
- Comment Entity
- Database Config
- Post Entity
- User Entity
- Notification Controller
- Notifier Configuration
- Notifier Port Interface
- GraphQL Query Generator
- Animal Entity Example
- Service Overrides
- Service Mocking
- Environment Loading Tasks
- Enum Branching Specification
- Environment Schema Binding
- GraphQL Support Design
- Client Proxy Service
- GraphQL Mutations
- GraphQL Resolvers
- GraphQL Subscriptions
- SSE Transport
- WebSocket Transport
- Duration Branching Specification
- Event Emitter Specification
- Environment Binding Context
- GraphQL Realtime Protocols
- GraphQL Support Context
- Logger Specification
- Metadata Registration Design
- Middleware Design
- Module Lazy Loading
- Module Re-export Design
- Multipart Form Streaming
- Numeric Boolean Branching
- Unified Parse API

## God Nodes (most connected - your core abstractions)
1. `New()` - 120 edges
2. `PropertyBuilder` - 79 edges
3. `HttpException` - 62 edges
4. `ProviderRef` - 59 edges
5. `Request` - 56 edges
6. `Context` - 56 edges
7. `Route` - 56 edges
8. `New()` - 51 edges
9. `New()` - 49 edges
10. `newBuiltin()` - 45 edges

## Surprising Connections (you probably didn't know these)
- `MustNewApp()` --calls--> `gonest.Module`  [EXTRACTED]
  gonest.go → README.md
- `SyncAccessorFields()` --conceptually_related_to--> `Accessor Dirty-Tracking`  [INFERRED]
  gonest.go → README.md
- `MustParse()` --calls--> `gonest.Schema`  [EXTRACTED]
  gonest.go → README.md
- `objectSchemaFor()` --references--> `ObjectSchema`  [EXTRACTED]
  .examples/full-text-search/person/dto.go → internal/schema/object.go
- `NewPersonProps()` --calls--> `NewAccessor()`  [INFERRED]
  .examples/full-text-search/shared/entity/entity.go → gonest.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Unified Transport Abstraction** — gonest_parseable, gonest_httpcontext, gonest_grpccontext, gonest_microservicecontext [EXTRACTED 1.00]
- **Core DI System** — gonest_module, gonest_provider, gonest_controller [EXTRACTED 1.00]
- **Milestone 19: Config Loading** — specs_features_dotenv_loading_tasks, specs_features_env_schema_binding_spec, specs_features_env_schema_binding_design, specs_features_env_schema_binding_tasks [EXTRACTED 1.00]
- **GraphQL Realtime Protocols & Support** — specs_features_graphql_support_spec, specs_features_graphql_realtime_protocols_spec, specs_features_graphql_realtime_protocols_design, specs_features_graphql_realtime_protocols_tasks [INFERRED 0.90]
- **GraphQL Support Subsystem** — internal_gqlresolver_resolver, internal_graphqlgen_generate, internal_gqltransport_sse, internal_gqltransport_ws [EXTRACTED 1.00]
- **Lifecycle Hooks Implementation Flow** — specs_features_lifecycle_hooks_tasks_t1, specs_features_lifecycle_hooks_tasks_t2, specs_features_lifecycle_hooks_tasks_t3, specs_features_lifecycle_hooks_tasks_t4, specs_features_lifecycle_hooks_tasks_t5, specs_features_lifecycle_hooks_tasks_t6, specs_features_lifecycle_hooks_tasks_t7 [EXTRACTED 1.00]

## Communities (248 total, 57 thin omitted)

### Community 0 - "Provider Lifecycle Testing"
Cohesion: 0.06
Nodes (65): callAndStore(), Provider, T, newResolvedProvider(), registerFuncs(), registerSignalFuncs(), TestHookRegistration_InvalidShapes(), TestHookRegistration_PanicMessage_NamesProviderTypeAndReceivedSignature() (+57 more)

### Community 1 - "Application Shutdown Sequence"
Cohesion: 0.05
Nodes (67): abLeafType, abMxHookedType, abMxUnhookedType, abP1Type, abP2Type, abP3Type, abRootType, applicationBootstrapRunner (+59 more)

### Community 2 - "Route Schema Testing"
Cohesion: 0.07
Nodes (50): New(), Reader, Schema, T, Type, Value, Writer, newFakeResponder() (+42 more)

### Community 3 - "Dotenv Environment Loading"
Cohesion: 0.07
Nodes (60): Dotenv, envPair, envParseIntoFixture, Get(), T, TestDotenv_ParseInto_DelegatesToParseEnvInto(), TestDotenv_SatisfiesParseable(), TestGet_ReturnsSameSingletonInstance() (+52 more)

### Community 4 - "HTTP Request Mocking"
Cohesion: 0.06
Nodes (31): barExampleError, fakeResponder, Filter, fooExampleError, filteredHandler(), findCatch(), Type, Value (+23 more)

### Community 5 - "Service Integration Testing"
Cohesion: 0.07
Nodes (53): authService, Dotenv(), idParams, insightListUsersQuery, insightPingable, insightPostgres, insightRedis, insightTestIUserService (+45 more)

### Community 6 - "Event Emitter System"
Cohesion: 0.07
Nodes (37): barEvent, fooEvent, Listener, Listener[EventType], quxEvent, subscribeTestEvent, Emitter, Mutex (+29 more)

### Community 7 - "Logging Configuration"
Cohesion: 0.09
Nodes (33): GetLogger(), GetLoggerFor(), Active(), Configure(), Debug(), defaultAllowed(), Error(), GetLogger() (+25 more)

### Community 8 - "Pipeline Dispatch Testing"
Cohesion: 0.12
Nodes (50): graphqlUserEntity, pipelineIDParams, New(), buildPipelineOrderingApp(), dispatchTestApp(), barFilterException, fooFilterException, nameParams (+42 more)

### Community 9 - "OpenAPI Schema Generation"
Cohesion: 0.13
Nodes (47): Generate(), schemaFor(), containsNull(), Schema, T, keys(), newUserSchema(), TestDocument_ProducesValidJSONMarshalableOutput() (+39 more)

### Community 10 - "Request Validation Exceptions"
Cohesion: 0.14
Nodes (44): BadRequestException, NewBadRequestException(), expectBadRequest(), T, mustNotNullableBody(), newCtx(), nullableRequiredValidBody(), TestMustJsonBody_AdditionalProperties_NoStructuralValidation() (+36 more)

### Community 11 - "HTTP Server Adapters"
Cohesion: 0.06
Nodes (14): controllableListenAdapter, fakeRegisteredRoute, fakeShutdownAdapter, listenSpyAdapter, Options, recordingFakeAdapter, fiberMethod(), TestFiberMethod_UnknownHttpMethod_Panics() (+6 more)

### Community 12 - "Dependency Injection Management"
Cohesion: 0.12
Nodes (41): fakePinger, fakeService, otherService, pingable, TestMustInjectAll_FromProviderOwner_DispatchesToMustAllProvider(), findAllRefs(), Module, MarkResolving() (+33 more)

### Community 13 - "Controller Declaration Testing"
Cohesion: 0.12
Nodes (37): ctrlAnimal, ctrlCat, ctrlFooService, New(), T, TestBearerAuth_SetsFlag(), TestController_SatisfiesModuleControllerRef(), TestController_SatisfiesModuleOwner() (+29 more)

### Community 14 - "Task Scheduler Module"
Cohesion: 0.11
Nodes (24): Duration, Module, Mutex, Once, Type, Value, New(), newJobHandle() (+16 more)

### Community 15 - "App Listener Testing"
Cohesion: 0.10
Nodes (36): consumerMarker, OnListen, displayAddr(), barFilterException, fooFilterException, nameParams, userIDNameParams, userIDParams (+28 more)

### Community 16 - "Search Query Schema"
Cohesion: 0.08
Nodes (30): collectFieldNames(), FieldNames(), FieldsSchemaFor(), Schema, T, Type, LikeMatch(), MatchField() (+22 more)

### Community 17 - "Module Assembly Logic"
Cohesion: 0.16
Nodes (35): New(), T, TestAssemble_AutoWiresOwnerModuleOnControllers(), TestAssemble_AutoWiresOwnerModuleOnProviders(), TestAssemble_DiamondImport_VisitsSharedModuleOnce(), TestAssemble_ExportDeclaredInProviders_NoError(), TestAssemble_ExportNotDeclaredInProviders_ReturnsError(), TestAssemble_SimpleBFS_VisitsImportedModule() (+27 more)

### Community 18 - "Array Schema Validation"
Cohesion: 0.14
Nodes (36): addressEntity, Schema, T, newAddressTestSchema(), newArrayTestSchema(), TestArray_CalledTwice_ProducesIndependentItemState(), TestArray_SetsFormatAndReturnsNewArraySchema(), TestArraySchema_FieldMethods_NeverMutateItem() (+28 more)

### Community 19 - "Route Metadata Definition"
Cohesion: 0.07
Nodes (5): Schema, Type, Value, resolver, Route

### Community 20 - "Module Internal References"
Cohesion: 0.09
Nodes (13): effectiveExports(), Module, TestModule_OwnControllers_ReturnsCopyNotInternalSlice(), TestModule_OwnControllers_ReturnsRegisteredControllers(), TestModule_OwnResolvers_ReturnsCopyNotInternalSlice(), TestModule_OwnResolvers_ReturnsRegisteredResolvers(), ControllerRef, FilterRef (+5 more)

### Community 21 - "Property Builder Fluent API"
Cohesion: 0.06
Nodes (4): addDescriptionAndExamples(), applyNullable(), Schema, PropertyBuilder

### Community 22 - "Parameter Coercion Logic"
Cohesion: 0.17
Nodes (25): NewBadRequestException(), coerceParamString(), Schema, StructField, Type, Value, int64SliceContains(), isNumericKind() (+17 more)

### Community 23 - "Dirty-Tracking Accessors"
Cohesion: 0.08
Nodes (31): Accessor Dirty-Tracking, gonest.Consumer, Emitter, EventType, Schema, T, Value, gonest.GrpcContext (+23 more)

### Community 24 - "HTTP Middleware Pipeline"
Cohesion: 0.10
Nodes (11): HttpContext, Guard, Interceptor, Next, composeHandler(), interceptedHandler(), Module, Module (+3 more)

### Community 25 - "Standard HTTP Exceptions"
Cohesion: 0.09
Nodes (30): BadGatewayException, ExpectationFailedException, GoneException, LengthRequiredException, NotAcceptableException, NotImplementedException, ProxyAuthRequiredException, RequestURITooLongException (+22 more)

### Community 26 - "Application Bootstrap Utilities"
Cohesion: 0.14
Nodes (29): Module, insightConnectable, insightConnectableService, MustNewApp(), NewApp(), NewController(), NewLoggerMiddleware(), NewModule() (+21 more)

### Community 27 - "WebSocket Protocol Testing"
Cohesion: 0.23
Nodes (23): closeCall, protoFakeWSConn, ackConn(), Schema, T, newProtoFakeWSConn(), newProtoMultiSubTestSchema(), newProtoSubTestSchema() (+15 more)

### Community 28 - "HTTP Responder Mock"
Cohesion: 0.10
Nodes (13): fakeResponder, New(), Reader, T, Writer, newFakeResponder(), TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(), TestDeclare_ExecutesFn() (+5 more)

### Community 30 - "Dependency Resolution Engine"
Cohesion: 0.13
Nodes (27): PendingAllEdges(), Resolve(), T, TestInvokeAndCopy_PendingAllEdge_WritesMatchedNodeIntoItsSlot(), TestResolve_ConstructorError_CancelsSiblingGoroutines(), TestResolve_ConstructorPanic_IsRecoveredAsError(), TestResolve_ConstructorReceivesConfiguredContext(), TestResolve_CopyInPlace_PlaceholderReflectsRealData() (+19 more)

### Community 31 - "Middleware Handler Testing"
Cohesion: 0.10
Nodes (13): New(), Reader, T, Writer, newFakeResponder(), TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(), TestDeclare_ExecutesFn(), TestDeclare_NilFn_DoesNotPanic() (+5 more)

### Community 32 - "Bootstrap Order Tracking"
Cohesion: 0.12
Nodes (24): orderLog, tpProviderA, tpProviderB, tpProviderC, tpProviderX, tpProviderY, customScalarEntity, scalarEntity (+16 more)

### Community 33 - "Fiber Web Framework Adapter"
Cohesion: 0.07
Nodes (7): App, fiberResponder, Ctx, Reader, Writer, init(), RegisterTestAdapter()

### Community 34 - "HTTP Response Mocking"
Cohesion: 0.11
Nodes (13): fakeResponder, New(), Reader, T, Writer, newFakeResponder(), TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(), TestDeclare_ExecutesFn() (+5 more)

### Community 35 - "Fiber Integration Testing"
Cohesion: 0.19
Nodes (28): New(), T, TestFiberWSConn_CloseWithCode_SendsRealCloseFrame(), TestInit_CalledTwice_DoesNotResetExistingApp(), TestInit_ZeroValueFiberApp_BecomesUsable(), TestListen_NilOnListen_DoesNotPanicAndBlocksNormally(), TestListen_OnListenFires_BeforeBlockingForGood(), TestNew_RegisterRoute_NoError() (+20 more)

### Community 36 - "Direct Provider Resolution"
Cohesion: 0.17
Nodes (25): animalIfaceType(), fakeProvider, T, Type, Value, newResolved(), TestFindDirect_ImportedNotReExported_StillInvisible(), TestFindDirect_Interface_ExactMatchTakesPrecedenceOverImplements() (+17 more)

### Community 38 - "Lifecycle Hook Invocation"
Cohesion: 0.18
Nodes (12): Context, describeGivenFunc(), Provider, Provider, Type, Value, invokeHook(), isValidHookSignature() (+4 more)

### Community 39 - "GraphQL Resolver Definition"
Cohesion: 0.11
Nodes (7): Resolver, Subscription, Module, Mutation, Query, Schema, newSubscription()

### Community 40 - "GraphQL Operation Registry"
Cohesion: 0.10
Nodes (17): graphqlPostBodyPeek, graphqlRequestBody, graphqlResponseBody, resolvableResolver, reservation, ReservationRegistry, sseSingleFrame, sseSingleOperationBody (+9 more)

### Community 41 - "Injection Ordering Tests"
Cohesion: 0.10
Nodes (16): mustInjectAllCacheAdapter, mustInjectAllOrderingAdapterA, mustInjectAllOrderingAdapterB, mustInjectAllOrderingAdapterC, mustInjectAllOrderingLog, mustInjectAllPingable, mustInjectAllSQLAdapter, mustInjectAllTransientAdapter (+8 more)

### Community 43 - "App Routing Pipeline"
Cohesion: 0.14
Nodes (19): declarable, pipelineStageType, routableController, asFilters(), asMiddleware(), countTree(), declareControllers(), declareListeners() (+11 more)

### Community 44 - "Request Validation Helpers"
Cohesion: 0.18
Nodes (15): Request, Reader, Schema, T, mustParseForm(), mustParseHeaders(), mustParseJSON(), mustParseParams() (+7 more)

### Community 45 - "Service Provider Mocking"
Cohesion: 0.13
Nodes (20): Provider, insightSchedulerUserService, NewProvider(), NewScheduler(), routeMustInjectUsecase, Module, newInsightHealthModule(), newInsightTestUserModule() (+12 more)

### Community 46 - "Provider Reference Metadata"
Cohesion: 0.11
Nodes (8): Module, Type, TestProviderRef_ResolvedType_ExposesUnderlyingType(), fakeController, fakeListener, fakeProvider, fakeResolver, fakeScheduler

### Community 47 - "Schema Property Registration"
Cohesion: 0.20
Nodes (23): Schema, T, Time, newTestSchema(), TestNew_NonStructTypePanics(), TestOwnProperties_ReturnsAllRegisteredFields(), TestOwnProperties_ReturnsCopyNotInternalSlice(), TestProperty_DoesNotSwapNeighboringFields() (+15 more)

### Community 48 - "Direct Dependency Lookup"
Cohesion: 0.17
Nodes (15): Type, Value, Type, Value, Type, Value, candidateProviders(), FindDirect() (+7 more)

### Community 49 - "SSE Responder Mock"
Cohesion: 0.09
Nodes (5): fakeSSEResponder, Reader, Writer, PipeReader, PipeWriter

### Community 50 - "Direct Dispatch Testing"
Cohesion: 0.16
Nodes (17): directIface, directImpl, fakeDirectOwner, Module, T, Type, Value, TestMustInject_DirectResolver_Interface_SingleMatch_Resolves() (+9 more)

### Community 51 - "OpenAPI Document Builder"
Cohesion: 0.25
Nodes (21): buildResponses(), defaultErrorResponse(), defaultExceptionName(), formBodySchemaObject(), Module, OpenAPI, Schema, StructField (+13 more)

### Community 52 - "Request Body Parsing"
Cohesion: 0.12
Nodes (8): BodySource, Parseable, Responder, Schema, T, mustParse(), newRequest(), NewGraphqlContext()

### Community 53 - "Dependency Graph Traversal"
Cohesion: 0.23
Nodes (20): PendingEdge, PendingEdges(), allProviders(), callConstructor(), edgesFor(), Module, Type, Value (+12 more)

### Community 54 - "JSON Sync Logic"
Cohesion: 0.22
Nodes (18): dirtyValue, PersonEntity, PersonProps, RawPerson, applyToDst(), collectDirtyAccessors(), collectDirtyAccessorsRecursive(), getAccessorValue() (+10 more)

### Community 55 - "Exception Name Mapping"
Cohesion: 0.23
Nodes (17): Exception, ExceptionName(), EffectiveName(), NewHttpException(), T, TestEffectiveName_FallsBackToConcreteTypeName_WhenNameUnset(), TestEffectiveName_ReturnsSetNameWhenNonEmpty(), TestFooExampleError_SatisfiesException() (+9 more)

### Community 56 - "GraphQL Field Builder"
Cohesion: 0.25
Nodes (12): Field, FieldConfigArgument, FieldResolveFn, builder, argKey(), fieldResolver(), Schema, identityScalarConfig() (+4 more)

### Community 57 - "SSE Connection Handling"
Cohesion: 0.39
Nodes (18): NewHttpContext(), NewReservationRegistry(), newFakeSSEResponder(), SSESingleConnectHandler(), attachAndDrainToken(), Reader, T, newSSESingleTestSchema() (+10 more)

### Community 59 - "Interface Provider Validation"
Cohesion: 0.16
Nodes (15): isProviderAsView, providerAsValidateAnimal, providerAsValidateCat, providerAsValidateDog, declareProviders(), Module, T, TestMustNewApp_ProviderAs_ConcreteMissing_Panics() (+7 more)

### Community 60 - "HTTP Exception Details"
Cohesion: 0.11
Nodes (10): fooExampleError, HttpException, HTTPVersionNotSupportedException, NotFoundException, RequestEntityTooLargeException, customTestException, NewHTTPVersionNotSupportedException(), NewRequestEntityTooLargeException() (+2 more)

### Community 61 - "Service Logging Spies"
Cohesion: 0.14
Nodes (7): gonestTestSpyLogger, insightEmitterUserService, insightHealthDb, insightLoggerService, Emitter, Mutex, TestEmitter_RootAlias_InsightUserCreatedExample()

### Community 62 - "GraphQL Schema Building"
Cohesion: 0.24
Nodes (18): genEmailOnlyEntity, genPostEntity, genUserEntity, Build(), Mutation, Query, Schema, T (+10 more)

### Community 63 - "Module Export Validation"
Cohesion: 0.25
Nodes (16): assemble(), Module, moduleName(), validateExports(), T, TestAssemble_ExportModulesImported_NoError(), TestAssemble_ExportModulesNotImported_ReturnsError(), TestModule_EffectiveExports_Diamond_DedupesSharedReExportedModule() (+8 more)

### Community 64 - "Provider Resolution Testing"
Cohesion: 0.27
Nodes (18): Find(), barType(), bazType(), fooType(), T, TestFind_DiamondImport_DirectImporterResolvesSharedProvider(), TestFind_DiamondImport_ResolvesDirectImportOnly(), TestFind_ImportedButNotExported_Panics() (+10 more)

### Community 65 - "Numeric Constraint Testing"
Cohesion: 0.27
Nodes (18): Schema, T, newNumericTestSchema(), TestBoolean_CommonConstraintsWork(), TestBoolean_ReturnsSamePropertyBuilder(), TestBooleanThenInteger_NoPanicLastWins(), TestIntegerThenBoolean_NoPanicLastWins(), TestNumericFamilyBranches_CalledTwiceLastWins() (+10 more)

### Community 66 - "Parameter Mocking Helpers"
Cohesion: 0.11
Nodes (3): Reader, Writer, paramFakeResponder

### Community 67 - "Built-in Auth Exceptions"
Cohesion: 0.17
Nodes (17): builtinCase, ConflictException, ForbiddenException, UnauthorizedException, NewConflictException(), NewForbiddenException(), NewUnauthorizedException(), gatedHandler() (+9 more)

### Community 68 - "Request Source Testing"
Cohesion: 0.36
Nodes (16): New(), T, newFakeResponder(), TestBodySource_Raw_EmptyByDefault(), TestBodySource_Raw_ReturnsRawBytes(), TestBodySource_Text_ReturnsStringOfRaw(), TestRequest_Body_ReturnsBodySource(), TestRequest_Header_ReadsFromResponder() (+8 more)

### Community 69 - "Dependency Injection Core"
Cohesion: 0.19
Nodes (17): allAsView, constructable, declarable, directResolver, lazyResolvedSetter, lazyScoped, PendingAllEdge, GlobalSingletonFor() (+9 more)

### Community 70 - "Schema Field Reflection"
Cohesion: 0.16
Nodes (5): cumulativeOffset(), findFieldByOffset(), Schema, StructField, Type

### Community 71 - "Form Data Mocking"
Cohesion: 0.12
Nodes (3): Reader, Writer, formFakeResponder

### Community 72 - "Route Parameter Validation"
Cohesion: 0.22
Nodes (16): T, newParamCtx(), TestMustParams_CustomFunc_ReceivesRawString_NotCoerced(), TestMustParams_FieldWithNoRouteMatch_ProducesViolation(), TestMustParams_HappyPath_TwoParams(), TestMustParams_MismatchedSchema_PanicsBeforeReadingAnyParam(), TestMustParams_PresentButInvalid_ProducesViolation(), TestMustParams_RealHTTPDispatch_CustomFunc() (+8 more)

### Community 73 - "Query String Mocking"
Cohesion: 0.11
Nodes (3): Reader, Writer, queryFakeResponder

### Community 74 - "Fiber HTTP Responder"
Cohesion: 0.12
Nodes (3): Ctx, Writer, httpFiberResponder

### Community 75 - "Test App Harness"
Cohesion: 0.16
Nodes (12): HttpAdapter, httpAdapterPtr, TestBuilder, MustNewTestApp(), T, Test, Module, T (+4 more)

### Community 76 - "HTTP Response Utilities"
Cohesion: 0.12
Nodes (3): fakeResponder, Reader, Writer

### Community 77 - "Mock Service Entities"
Cohesion: 0.12
Nodes (14): FooExampleError, insightHealthRedis, insightTestUserEntity, insightTestUserService, insightTestUserServiceMock, NewFilter(), NewHttpException(), NewNotFoundException() (+6 more)

### Community 78 - "Request Parameter Mocking"
Cohesion: 0.12
Nodes (3): paramFakeResponder, Reader, Writer

### Community 79 - "Query Parameter Validation"
Cohesion: 0.22
Nodes (15): NewBodySource(), T, newQueryCtx(), TestMustQuery_CustomFunc_ReceivesRawString_NotCoerced(), TestMustQuery_HappyPath_TwoParams(), TestMustQuery_MismatchedSchema_PanicsBeforeReadingAnyQuery(), TestMustQuery_MissingRequiredAndOutOfRange_BothCollected(), TestMustQuery_RealHTTPDispatch_CustomFunc() (+7 more)

### Community 80 - "Duration Validation Testing"
Cohesion: 0.22
Nodes (16): Duration, T, TestDuration_Env_Absent_UsesTypedDefault(), TestDuration_Env_Present_ParsesStringValue(), TestDuration_JSONBody_AboveMax_ProducesFieldViolation(), TestDuration_JSONBody_BelowMin_ProducesFieldViolation(), TestDuration_JSONBody_EnumAllowedValue_Populates(), TestDuration_JSONBody_EnumViolation() (+8 more)

### Community 82 - "Circular Dependency Detection"
Cohesion: 0.25
Nodes (14): ProviderAs(), cycleError(), DetectCycle(), T, TestDetectCycle_DirectCycle_ReturnsFullChain(), TestDetectCycle_DisconnectedAcyclicComponents_NoFalsePositive(), TestDetectCycle_IndirectCycle_ReturnsFullChainNotJustFoundCycle(), TestDetectCycle_NoCycle_ReturnsNil() (+6 more)

### Community 83 - "GraphQL Mutation Definition"
Cohesion: 0.17
Nodes (5): Query, Mutation, newMutation(), Schema, newQuery()

### Community 84 - "Provider Type Casting"
Cohesion: 0.15
Nodes (7): Module, Type, Value, ProviderAs(), hasResolvedValue, isProviderAsView, providerAsRef

### Community 86 - "Request Scoped Cache"
Cohesion: 0.22
Nodes (12): Mutex, NewRequestScope(), requestIDFrom(), T, TestNewRequestScope_ReturnsUsableCache(), TestRequestScope_DifferentContexts_ReturnDifferentInstances(), TestRequestScope_Get_NoRequestIDOnContext_ReportsNotFound(), TestRequestScope_SameContext_ReturnsSameInstance() (+4 more)

### Community 87 - "String Constraint Testing"
Cohesion: 0.32
Nodes (15): Schema, T, newStringTestSchema(), TestPropertyBuilder_FormatValueDefaultsEmpty(), TestStringFamilyBranches_CalledTwiceLastWins(), TestStringFamilyBranches_SetsCorrectFormat(), TestStringSchema_CommonConstraintsMutateSharedBuilderAndStayChainable(), TestStringSchema_EnumCalledTwiceLastWins() (+7 more)

### Community 88 - "Swagger Documentation Setup"
Cohesion: 0.23
Nodes (13): main(), main(), App, OpenAPI, MustSetupSwagger(), NewOpenAPI(), OpenapiGenerate(), SetupSwagger() (+5 more)

### Community 89 - "Extended HTTP Exceptions"
Cohesion: 0.14
Nodes (15): MethodNotAllowedException, PaymentRequiredException, PreconditionRequiredException, TooEarlyException, NewMethodNotAllowedException(), NewPaymentRequiredException(), NewPreconditionRequiredException(), NewTooEarlyException() (+7 more)

### Community 90 - "GraphQL Execution Logic"
Cohesion: 0.20
Nodes (12): wsProtocolMessage, wsSubscribePayload, Execute(), Schema, T, newExecTestSchema(), TestExecute_InvalidQuery_ReturnsErrors(), TestExecute_ValidQuery_ReturnsData() (+4 more)

### Community 91 - "Accessor State Sync"
Cohesion: 0.28
Nodes (13): New(), T, TestApply_WritesOnlyWhenDirty(), TestMarshalJSON_EmitsInnerValueDirectly(), TestNew_WithArg_DirtyAndValueSet(), TestNew_WithoutArgs_NotDirty(), TestOnDirty_CalledOnlyWhenDirty(), TestSet_MarksDirtyAndStoresValue() (+5 more)

### Community 92 - "Distinct SSE Streams"
Cohesion: 0.27
Nodes (13): Schema, SSEDistinctHandler(), streamSSEDistinctSubscription(), T, newSSEDistinctTestSchema(), TestSSEDistinctHandler_ClientDisconnects_HandlerGoroutineEnds(), TestSSEDistinctHandler_InvalidQuery_RespondsNextWithErrorNot400(), TestSSEDistinctHandler_Subscription_EmitsNextPerEmittedValue() (+5 more)

### Community 93 - "Provider Casting Tests"
Cohesion: 0.21
Nodes (12): fakeProvider, T, Value, TestProviderAs_ChainingAnotherProviderAsView_Panics(), TestProviderAs_NonInterfaceT_Panics(), TestProviderAs_ReportsTAsResolvedType(), TestProviderAs_ResolvedValue_DelegatesToWrappedRef(), TestProviderAs_ResolvedValue_NoDelegate_ReturnsFalse() (+4 more)

### Community 94 - "Dependency Graph Visualization"
Cohesion: 0.25
Nodes (12): BuildGraph(), Module, T, resetForGraphTest(), TestBuildGraph_ExcludesControllerOwnedEdges(), TestBuildGraph_IncludesPendingAllEdges_AlongsidePendingEdges(), TestBuildGraph_NodeWithNoDependenciesHasEmptyList(), TestBuildGraph_SingleDependencyEdge() (+4 more)

### Community 95 - "Value Schema Definition"
Cohesion: 0.21
Nodes (13): Schema, Type, NewValue(), Schema, T, Type, newTestValueSchema(), TestIsValue_FalseForStructShapedSchema() (+5 more)

### Community 96 - "Data Transfer Objects"
Cohesion: 0.20
Nodes (11): Accessor, objectSchemaFor(), Time, BodyCreateDTO, BodyUpdateDTO, ParamsDTO, QueryDTOWhere, MatchBool (+3 more)

### Community 97 - "Lifecycle Event Recording"
Cohesion: 0.27
Nodes (10): e2eLifecycleRecorder, e2eLifecycleType, e2eLifecycleTypeNoHooks, assertLog(), Mutex, Provider, T, newE2ELifecycleProvider() (+2 more)

### Community 98 - "Entity CRUD Interfaces"
Cohesion: 0.29
Nodes (12): Creatable, Deletable, Indexable, PersonProps, Updatable, Time, NewCreatable(), NewDeletable() (+4 more)

### Community 99 - "Service Layer Implementation"
Cohesion: 0.33
Nodes (8): Person, applySort(), Mutex, matchWhere(), paginationBounds(), sortLess(), Service, QueryDTO

### Community 100 - "Todo Service Logic"
Cohesion: 0.20
Nodes (5): Mutex, TodoEntity, TodoService, TodoStats, TodoStatsUsecase

### Community 101 - "GraphQL Request Handling"
Cohesion: 0.27
Nodes (12): registerRoutes(), Schema, graphqlHandler(), graphqlPostDispatcher(), Schema, SSESingleOperationHandler(), NewFormBodySource(), NewHeadersSource() (+4 more)

### Community 102 - "Environment Variable Parsing"
Cohesion: 0.25
Nodes (12): ParseEnvInto(), T, TestParseEnvInto_AllFieldsSet_PopulatesCorrectly(), TestParseEnvInto_DefaultUsedWhenAbsent_RealValueUsedWhenPresent(), TestParseEnvInto_EmptyButSetEnvVar_TreatedAsPresent(), TestParseEnvInto_FieldWithoutEnvTag_Ignored(), TestParseEnvInto_IntegerCoercionFails_RecordsViolation(), TestParseEnvInto_TwoRequiredMissing_CollectsBothViolations() (+4 more)

### Community 103 - "Multipart Form Parsing"
Cohesion: 0.35
Nodes (13): buildMultipartBody(), Buffer, T, newFormCtx(), TestMustFormBody_PanicsOnError(), TestParseFormBody_CustomFunc_ReceivesRawString_NotCoerced(), TestParseFormBody_HappyPath_FieldAndFile(), TestParseFormBody_MalformedMultipartBody_ReturnsOneViolation() (+5 more)

### Community 106 - "Duration Validation Tests"
Cohesion: 0.44
Nodes (12): Duration, Schema, T, newDurationTestSchema(), TestDuration_SetsCorrectFormatAndKind(), TestDurationSchema_CommonConstraintsMutateSharedBuilderAndStayChainable(), TestDurationSchema_EnumChainAndRoundTrip(), TestDurationSchema_EnumDefaultUnset() (+4 more)

### Community 107 - "Schema Kind Validation"
Cohesion: 0.37
Nodes (12): Schema, T, Time, newKindAddressTestSchema(), newKindTestSchema(), TestKindValue_ArrayItemBranches_MirrorPropertyBuilder(), TestKindValue_BooleanAndString_AreDifferent(), TestKindValue_EveryBranch() (+4 more)

### Community 108 - "App Lifecycle Hooks"
Cohesion: 0.27
Nodes (10): wiringFailingBootstrapType, wiringFailingInitType, wiringHookedType, T, TestListen_AdapterListenError_ReturnsImmediatelyWithoutWaitingOnShutdownDone(), TestListen_ShutdownHooksDisabled_ReturnsAsSoonAsAdapterListenReturns(), TestListen_ShutdownHooksEnabled_BlocksUntilShutdownDoneThenReturnsShutdownErr(), TestNewApp_ApplicationBootstrapHookError_AbortsNewApp() (+2 more)

### Community 109 - "User Controller Integration"
Cohesion: 0.33
Nodes (7): UserEntity, UserService, TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(), TestNewApp_UserControllerRealHttpClient_EndToEndOverRealPort(), TestNewApp_ZeroFilters_NonRegressionReference(), TestNewApp_ZeroGuards_NonRegressionReference(), TestNewApp_ZeroInterceptors_NonRegressionReference()

### Community 110 - "OpenAPI Metadata Configuration"
Cohesion: 0.30
Nodes (11): T, TestBearerAuth_SetsFlag(), TestContact_SetsAndOverwrites(), TestDescription_SetsAndOverwrites(), TestLicense_SetsAndOverwrites(), TestNew_DefaultValues(), TestNew_InsightBootstrapExample(), TestNew_NilFn_DoesNotPanic() (+3 more)

### Community 111 - "Swagger UI Integration"
Cohesion: 0.30
Nodes (10): App, OpenAPI, renderSwaggerUIHTML(), SetupSwagger(), T, TestRenderSwaggerUIHTML_DifferentOptions_ProduceDifferentOutput(), TestRenderSwaggerUIHTML_InterpolatesOptions(), TestSetupSwagger_EmptyDocument_StillReturnsValidJSON() (+2 more)

### Community 112 - "DateTime Schema Validation"
Cohesion: 0.36
Nodes (11): Schema, T, Time, newDateTimeTestSchema(), TestDate_ReturnsSamePropertyBuilder(), TestDateThenDateTime_NoPanicLastWins(), TestDateTime_CommonConstraintsWork(), TestDateTime_InsightCreatedAtUpdatedAtDeletedAtChains() (+3 more)

### Community 114 - "Schema Type Registry"
Cohesion: 0.27
Nodes (10): Schema, Type, Lookup(), Register(), T, TestLookup_NeverRegisteredType_ReturnsFalseNoPanic(), TestNew_CalledTwiceForSameType_Panics(), TestRegister_ConcurrentDistinctTypes_NoRace() (+2 more)

### Community 115 - "Enum Validation Tests"
Cohesion: 0.35
Nodes (11): enumFixtureBody(), T, TestMustJsonBody_EnumAndPatternAndMin_AllViolationsCollected(), TestMustJsonBody_IntegerEnum_AllowedValue_Passes(), TestMustJsonBody_IntegerEnum_DisallowedValue_RecordsOneViolation(), TestMustJsonBody_NoEnumCall_AnyValueOfRightTypeStillPasses(), TestMustJsonBody_NullableEnumField_ExplicitNull_Accepted(), TestMustJsonBody_StringEnum_AllowedValue_Passes() (+3 more)

### Community 116 - "GraphQL Argument Normalization"
Cohesion: 0.26
Nodes (9): NewGraphqlArgsSource(), normalizeGraphqlValue(), Schema, T, newGraphqlArgsTestSchema(), TestNewGraphqlArgsSource_NormalizesNativeIntArg(), TestNewGraphqlArgsSource_NormalizesNestedIntArgs(), graphqlArgsEntity (+1 more)

### Community 117 - "Form File Uploads"
Cohesion: 0.22
Nodes (5): FormFile, Reader, NewFormFile(), Part, formBodySource

### Community 118 - "Schema Refinement Logic"
Cohesion: 0.42
Nodes (10): Schema, T, newSanitizeRefineTestSchema(), TestPropertyBuilder_Sanitize_LastCallWins(), TestPropertyBuilder_Sanitize_StoresFn_RetrievableViaSanitizeFunc(), TestPropertyBuilder_SanitizeFunc_NeverCalled_ReturnsFalse(), TestSchema_OwnRefines_EmptyByDefault(), TestSchema_OwnRefines_ReturnsCopyNotInternalSlice() (+2 more)

### Community 119 - "Custom Validation Functions"
Cohesion: 0.33
Nodes (10): decodeV1Code(), T, mustMarshal(), TestCustomFunc_SameDefinitionReused_ProducesSameResult(), TestMustJsonBody_CustomFunc_DecodesCustomFormat_EndToEnd(), TestMustJsonBody_CustomFunc_ReturningError_ProducesViolation_CollectedWithOthers(), TestMustJsonBody_CustomFunc_WrongGoType_ProducesViolation_NeverPanics(), TestMustJsonBody_FieldWithoutCustom_PopulatesExactlyAsBefore() (+2 more)

### Community 121 - "Blog GraphQL Service"
Cohesion: 0.29
Nodes (5): PostCreatedEvent, PostEntity, Service, Emitter, Mutex

### Community 122 - "User Service Exceptions"
Cohesion: 0.27
Nodes (5): DB, Entity, NewDuplicateEmailException(), DuplicateEmailException, Service

### Community 124 - "Lazy Dependency Injection"
Cohesion: 0.31
Nodes (9): lazyConfig, otherLazyConfig, T, TestMustLazy_NestedMustInject_Panics(), TestMustLazy_NoMatchingProvider_Panics(), TestMustLazy_NonPointerType_Panics(), TestMustLazy_NonSingletonProvider_Panics(), TestMustLazy_RepeatedCall_ReusesCachedValue() (+1 more)

### Community 125 - "GraphQL Protocol Testing"
Cohesion: 0.29
Nodes (10): App, Conn, Duration, Reader, listenOnEphemeralPort(), newPingOnlyApp(), readLineWithDeadline(), TestNewApp_GraphqlGet_NoUpgradeNoToken_DispatchesSSEDistinctHandler() (+2 more)

### Community 126 - "HTTP Reply Tests"
Cohesion: 0.36
Nodes (9): T, TestReply_Html_DelegatesToResponder(), TestReply_Json_DelegatesToResponder(), TestReply_Request_ReturnsOriginatingRequest(), TestReply_SetHeader_WritesToResponder(), TestReply_Status_IsChainableAndSetsCode(), TestReply_Status_Json_Chained(), TestReply_StatusCode_ReadsCurrentStatus() (+1 more)

### Community 127 - "GraphQL Resolver Declaration"
Cohesion: 0.47
Nodes (9): New(), T, TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(), TestDeclare_ExecutesFn(), TestNew_DoesNotExecuteFnOnCall(), TestOwnerModule_NilBeforeAssociation(), TestOwnerModule_PopulatedAfterSetOwnerModule(), TestResolver_QueryMutationSubscription_AccumulateAndRoundTrip() (+1 more)

### Community 128 - "Lazy Module Loading"
Cohesion: 0.36
Nodes (9): T, TestLazyModule_Exports_LandsOnOwnerModule(), TestLazyModule_Exports_ModuleRef_LandsOnOwnerModule(), TestLazyModule_Imports_LandsOnOwnerModule(), TestLazyModule_OwnProviders_DelegatesToOwner(), TestLazyModule_OwnProviders_IsDefensiveCopy(), TestModule_Lazy_NilFnIsNoOp(), TestModule_Lazy_RunsBeforeAssembleReadsImportsExports() (+1 more)

### Community 129 - "JSON Response Assertions"
Cohesion: 0.31
Nodes (5): TestResponse, Test, T, lookupJSONPath(), normalizeJSONValue()

### Community 131 - "Default Value Handling"
Cohesion: 0.47
Nodes (8): Schema, T, newDefaultTestSchema(), TestPropertyBuilder_Default_LastCallWins(), TestPropertyBuilder_Default_ReturnsSelfForChaining(), TestPropertyBuilder_Default_SetsDefaultValue(), TestPropertyBuilder_DefaultValue_NeverCalled_ReturnsFalse(), defaultEntity

### Community 132 - "Lazy Module Integration"
Cohesion: 0.46
Nodes (7): lazyDriverConfig, buildLazyDrivenApp(), App, T, TestLazyModule_ConfigProviderConstructorRunsExactlyOnce(), TestLazyModule_PicksModuleA_RealHttpDispatch(), TestLazyModule_PicksModuleB_RealHttpDispatch()

### Community 134 - "User Provider Resolution"
Cohesion: 0.50
Nodes (3): UserEntity, UserService, TestNewApp_UserProviderExample_ResolvesUsableUserService()

### Community 135 - "Lazy Module Definition"
Cohesion: 0.25
Nodes (3): Module, Module, LazyModule

### Community 136 - "Dependency Injection Tokens"
Cohesion: 0.36
Nodes (3): Module, Type, fakeProvider

### Community 137 - "HTTP Method Constants"
Cohesion: 0.43
Nodes (7): T, TestHttpDelete_String(), TestHttpGet_String(), TestHttpMethod_String_Unknown(), TestHttpPost_String(), TestHttpPut_String(), TestHttpQuery_String()

### Community 138 - "Custom Schema Properties"
Cohesion: 0.50
Nodes (7): Schema, T, newCustomTestSchema(), TestPropertyBuilder_Custom_LastCallWins(), TestPropertyBuilder_Custom_StoresFn_RetrievableViaCustomFunc(), TestPropertyBuilder_CustomFunc_NeverCalled_ReturnsFalse(), customEntity

### Community 139 - "Project Module List"
Cohesion: 0.25
Nodes (8): blog-api, blog-graphql, config-dotenv, full-text-search, gonest.dev/gonest, lifecycle-hooks, notification-driver, simple-todo

### Community 140 - "Event Emitter Subscription"
Cohesion: 0.38
Nodes (4): Emitter, T, Type, Subscribe()

### Community 141 - "Post Service Logic"
Cohesion: 0.43
Nodes (3): DB, Entity, Service

### Community 142 - "GraphQL Scalar Mapping"
Cohesion: 0.52
Nodes (6): Schema, T, newGraphqlScalarTestSchema(), TestPropertyBuilder_GraphqlScalar_StoresName_RetrievableViaGraphqlScalarValue(), TestPropertyBuilder_GraphqlScalarValue_NeverCalled_ReturnsFalse(), graphqlScalarEntity

### Community 143 - "Lifecycle Phase Orchestration"
Cohesion: 0.38
Nodes (7): T1: Provider no-signal lifecycle hooks, T2: Provider signal lifecycle hooks, T3: HttpAdapter.Shutdown + FiberApp.Shutdown, T4: internal/app bootstrap-time phase runners, T5: internal/app shutdown orchestrator, T6: Wire lifecycle hooks into NewApp/Listen, T7: End-to-end lifecycle test

### Community 144 - "Constructor Injection Constraints"
Cohesion: 0.40
Nodes (5): mustInjectInsideConstructorConsumer, mustInjectInsideConstructorDep, T, TestMustInject_CalledInsideConstructor_ProducesResolveError(), TestMustNewApp_CalledInsideConstructor_Panics()

### Community 145 - "Comment Service Logic"
Cohesion: 0.40
Nodes (3): Service, DB, Entity

### Community 146 - "Request Reservation Registry"
Cohesion: 0.53
Nodes (5): T, TestReservationRegistry_AttachThenRoute_ReturnsWriteFunc(), TestReservationRegistry_ConcurrentReserveAttachRoute_NoRace(), TestReservationRegistry_Reserve_ReturnsUniqueToken(), TestReservationRegistry_RouteBeforeAttach_ReturnsNotOk()

### Community 147 - "Module Export Resolution"
Cohesion: 0.67
Nodes (5): findExported(), findOwn(), Module, Type, hasOwnUnexported()

### Community 148 - "Post Data Transfer Objects"
Cohesion: 0.40
Nodes (4): CreateBodyDTO, ListQueryDTO, ParamsDTO, UploadAttachmentFormDTO

### Community 149 - "Exception Schema Shape"
Cohesion: 0.40
Nodes (4): schemaShape, Schema, T, newSchema()

### Community 150 - "App Options Configuration"
Cohesion: 0.60
Nodes (4): T, TestAppOptions_ZeroValue(), TestOnListen_Invocable(), TestOnListen_NilSafe()

### Community 151 - "Todo Controller DTOs"
Cohesion: 0.50
Nodes (3): createTodoBody, todoIDParams, updateTodoBody

### Community 152 - "Core Framework Types"
Cohesion: 0.50
Nodes (4): gonest.Controller, gonest.HttpContext, gonest.Module, gonest.Provider

### Community 153 - "Cross-Transport Event Testing"
Cohesion: 0.50
Nodes (3): orderCreatedEvent, T, TestWSProtocolAndSSEDistinct_SameSubscription_BothReceiveSameEmittedEvent()

### Community 162 - "Failed Dependency Exception"
Cohesion: 0.67
Nodes (3): FailedDependencyException, NewFailedDependencyException(), NewFailedDependencyException()

### Community 163 - "Gateway Timeout Exception"
Cohesion: 0.67
Nodes (3): GatewayTimeoutException, NewGatewayTimeoutException(), NewGatewayTimeoutException()

### Community 164 - "Insufficient Storage Exception"
Cohesion: 0.67
Nodes (3): InsufficientStorageException, NewInsufficientStorageException(), NewInsufficientStorageException()

### Community 165 - "Internal Server Error"
Cohesion: 0.67
Nodes (3): InternalServerErrorException, NewInternalServerErrorException(), NewInternalServerErrorException()

### Community 166 - "Locked Resource Exception"
Cohesion: 0.67
Nodes (3): LockedException, NewLockedException(), NewLockedException()

### Community 167 - "Loop Detected Exception"
Cohesion: 0.67
Nodes (3): LoopDetectedException, NewLoopDetectedException(), NewLoopDetectedException()

### Community 168 - "Misdirected Request Exception"
Cohesion: 0.67
Nodes (3): MisdirectedRequestException, NewMisdirectedRequestException(), NewMisdirectedRequestException()

### Community 169 - "Network Auth Exception"
Cohesion: 0.67
Nodes (3): NetworkAuthenticationRequiredException, NewNetworkAuthenticationRequiredException(), NewNetworkAuthenticationRequiredException()

### Community 170 - "Not Extended Exception"
Cohesion: 0.67
Nodes (3): NotExtendedException, NewNotExtendedException(), NewNotExtendedException()

### Community 171 - "Precondition Failed Exception"
Cohesion: 0.67
Nodes (3): PreconditionFailedException, NewPreconditionFailedException(), NewPreconditionFailedException()

### Community 172 - "Range Not Satisfiable"
Cohesion: 0.67
Nodes (3): RequestedRangeNotSatisfiableException, NewRequestedRangeNotSatisfiableException(), NewRequestedRangeNotSatisfiableException()

### Community 173 - "Header Too Large"
Cohesion: 0.67
Nodes (3): RequestHeaderFieldsTooLargeException, NewRequestHeaderFieldsTooLargeException(), NewRequestHeaderFieldsTooLargeException()

### Community 174 - "Request Timeout Exception"
Cohesion: 0.67
Nodes (3): RequestTimeoutException, NewRequestTimeoutException(), NewRequestTimeoutException()

### Community 175 - "Service Unavailable Exception"
Cohesion: 0.67
Nodes (3): ServiceUnavailableException, NewServiceUnavailableException(), NewServiceUnavailableException()

### Community 176 - "Teapot Exception"
Cohesion: 0.67
Nodes (3): TeapotException, NewTeapotException(), NewTeapotException()

### Community 177 - "Too Many Requests"
Cohesion: 0.67
Nodes (3): TooManyRequestsException, NewTooManyRequestsException(), NewTooManyRequestsException()

### Community 178 - "Unprocessable Entity Exception"
Cohesion: 0.67
Nodes (3): UnprocessableEntityException, NewUnprocessableEntityException(), NewUnprocessableEntityException()

### Community 179 - "Upgrade Required Exception"
Cohesion: 0.67
Nodes (3): UpgradeRequiredException, NewUpgradeRequiredException(), NewUpgradeRequiredException()

### Community 180 - "Variant Negotiates Exception"
Cohesion: 0.67
Nodes (3): VariantAlsoNegotiatesException, NewVariantAlsoNegotiatesException(), NewVariantAlsoNegotiatesException()

### Community 184 - "Filter Design Specs"
Cohesion: 0.67
Nodes (3): Filter Design, Filter Specification, Filter Tasks

### Community 185 - "GraphQL Realtime Specs"
Cohesion: 0.67
Nodes (3): GraphQL Realtime Protocols Design, GraphQL Realtime Protocols Specification, GraphQL Realtime Protocols Tasks

## Knowledge Gaps
- **234 isolated node(s):** `blog-api`, `CreateBodyDTO`, `ListQueryDTO`, `Entity`, `CreateBodyDTO` (+229 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **57 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `Pipeline Dispatch Testing` to `Bootstrap Order Tracking`, `Application Shutdown Sequence`, `Lifecycle Event Recording`, `Lazy Module Integration`, `GraphQL Request Handling`, `User Provider Resolution`, `Logging Configuration`, `GraphQL Operation Registry`, `Injection Ordering Tests`, `App Routing Pipeline`, `Dependency Injection Management`, `User Controller Integration`, `App Lifecycle Hooks`, `App Listener Testing`, `Constructor Injection Constraints`, `Interface Provider Validation`, `GraphQL Protocol Testing`, `Dependency Resolution Engine`?**
  _High betweenness centrality (0.170) - this node is a cross-community bridge._
- **Why does `Deregister()` connect `GraphQL Schema Building` to `Bootstrap Order Tracking`, `Numeric Constraint Testing`, `Route Schema Testing`, `Default Value Handling`, `Pipeline Dispatch Testing`, `OpenAPI Schema Generation`, `Custom Schema Properties`, `Duration Validation Tests`, `Schema Kind Validation`, `GraphQL Scalar Mapping`, `Schema Property Registration`, `DateTime Schema Validation`, `Array Schema Validation`, `Schema Type Registry`, `GraphQL Argument Normalization`, `Schema Refinement Logic`, `String Constraint Testing`, `Value Schema Definition`?**
  _High betweenness centrality (0.158) - this node is a cross-community bridge._
- **Why does `Context` connect `Lifecycle Hook Invocation` to `Application Shutdown Sequence`, `Fiber Web Framework Adapter`, `Event Emitter System`, `GraphQL Resolver Definition`, `HTTP Server Adapters`, `Service Provider Mocking`, `Task Scheduler Module`, `GraphQL Mutation Definition`, `Request Body Parsing`, `Dependency Graph Traversal`, `Request Scoped Cache`, `GraphQL Field Builder`, `WebSocket Protocol Testing`, `Service Logging Spies`, `Dependency Resolution Engine`?**
  _High betweenness centrality (0.108) - this node is a cross-community bridge._
- **Are the 106 inferred relationships involving `New()` (e.g. with `registerGraphql()` and `runApplicationBootstrapPhase()`) actually correct?**
  _`New()` has 106 INFERRED edges - model-reasoned connections that need verification._
- **Are the 27 inferred relationships involving `ProviderRef` (e.g. with `TestFindAllRefs_DiamondImport_Dedups()` and `TestFindAllRefs_ImportExportedMatch()`) actually correct?**
  _`ProviderRef` has 27 INFERRED edges - model-reasoned connections that need verification._
- **What connects `blog-api`, `CreateBodyDTO`, `ListQueryDTO` to the rest of the system?**
  _234 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Provider Lifecycle Testing` be split into smaller, more focused modules?**
  _Cohesion score 0.05877167205406994 - nodes in this community are weakly interconnected._