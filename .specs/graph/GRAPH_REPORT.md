# Graph Report - .  (2026-08-22)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 3552 nodes · 7424 edges · 318 communities (194 shown, 124 thin omitted)
- Extraction: 82% EXTRACTED · 18% INFERRED · 0% AMBIGUOUS · INFERRED: 1316 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `a473d1dc`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Provider Lifecycle Testing
- inject_test.go
- fiber_dispatch_test.go
- parse_test.go
- route_test.go
- validate.go
- Event Emitter System
- builtin.go
- logger.go
- fakeResponder
- gonest_test.go
- New
- validate_test.go
- fakeResponder
- fakeResponder
- HttpContext
- fakeResponder
- New
- Scheduler
- New
- search.go
- New
- newArrayTestSchema
- httpFiberResponder
- ws_protocol_test.go
- direct_test.go
- app.go
- Module
- PropertyBuilder
- Route
- Request
- T
- Bootstrap Order Tracking
- Controller
- fiber_test.go
- New
- Resolve
- New
- ArraySchema
- app/lifecycle_test.go
- fakeResponder
- Subscription
- gonest.go
- graphqlPostDispatcher
- Context
- Find
- must_inject_all_provider_test.go
- NewModule
- StringSchema
- HttpException
- Module
- schema_test.go
- builtinCases
- newInsightHealthModule
- direct_dispatch_test.go
- openapi/generate.go
- fakeSSEResponder
- sync.go
- NewHttpException
- builder
- paramFakeResponder
- NumericSchema
- HttpAdapter
- provider_as_validate_test.go
- TestGenerateOpenApiSchema_RootAlias_InsightExample
- fiberResponder
- assemble
- stage3.go
- newNumericTestSchema
- paramFakeResponder
- queryFakeResponder
- Schema
- formFakeResponder
- params_test.go
- wiring_test.go
- Options
- handleSubscribe
- NewBodySource
- app/lifecycle.go
- Query
- SSEDistinctHandler
- providerAsRef
- OpenAPI
- NewRequestScope
- newStringTestSchema
- validate/duration_test.go
- New
- provider_as_test.go
- ProviderRef
- BuildGraph
- newTestValueSchema
- Accessor
- lifecycle_e2e_test.go
- entity/entity.go
- Person
- TodoService
- DurationSchema
- form_test.go
- NewProvider
- Lookup
- FindDirectAll
- newDurationTestSchema
- New
- Reply
- openapi_test.go
- renderSwaggerUIHTML
- newDateTimeTestSchema
- ObjectSchema
- enum_test.go
- validate/testhelpers_test.go
- App
- newSanitizeRefineTestSchema
- validate/custom_test.go
- Accessor[T]
- HttpMethod
- Service
- Service
- FormFile
- module/lazy_test.go
- TestResponse
- fiberWSConn
- newDefaultTestSchema
- buildLazyDrivenApp
- DbService
- gonestTestSpyLogger
- .Get
- UserEntity
- method_test.go
- Response
- newCustomTestSchema
- lefthook.yaml
- gonest.dev/gonest
- Subscribe
- Service
- NewParamsSource
- fakeProvider
- newGraphqlScalarTestSchema
- T5: internal/app shutdown orchestrator
- must_inject_inside_constructor_test.go
- Service
- LockedException
- NewFooExampleError
- JSON Body Validation Tasks
- reservation_test.go
- post/dto.go
- newSchema
- gonest.Parseable
- options_test.go
- .ResolveDirect
- simple-todo/controller.go
- .ResolveDirect
- blog-graphql/dto.go
- comment/dto.go
- Service
- user/dto.go
- openDB
- Service
- InsufficientStorageException
- LengthRequiredException
- LoopDetectedException
- MisdirectedRequestException
- NotExtendedException
- PreconditionRequiredException
- RequestHeaderFieldsTooLargeException
- RequestTimeoutException
- TooManyRequestsException
- UnavailableForLegalReasonsException
- UnprocessableEntityException
- UnsupportedMediaTypeException
- jsonBodyUserEntity
- Response
- hasAnyBearerAuth
- internal/schema
- fakeFilter
- fakeMiddleware
- Filter Design
- GraphQL Realtime Protocols Specification
- Object Builder Tasks
- Entity
- DatabaseConfig
- post/entity.go
- user/entity.go
- notification-driver/controller.go
- notifier/config.go
- port.go
- Context (Legacy)
- Fiber Adapter Implementation
- Accessor (formerly Value)
- Dotenv Loading Tasks
- Enum Branches Specification
- Env -> Schema Binding Design
- GraphQL Support Design
- Interceptor Specification
- Lifecycle Hooks Context
- Middleware Specification
- Module Lazy Loading Design
- Module Re-export Specification
- Multipart Form Streaming Specification
- Numeric & Boolean Branches Design
- Provider Interface Export Design
- AD-002: Metadata builder é linear
- AD-008: Pipeline-stage types não suportam MustInject em v1
- AD-012: Storage de branch-wrapper relocado pro PropertyBuilder
- AD-051: Module.ExportModules transitivo
- blog-graphql README
- gonest.ClientProxy
- gonest.MustListen
- Emitter
- Mutex
- Time
- Writer
- App
- Conn
- Duration
- Metadata & PropertyBuilder
- StructField
- Type
- Value
- Duration Branch Specification
- Emitter Specification
- Env -> Schema Binding Context
- GraphQL Realtime Protocols Context
- GraphQL Support Context
- HTTP Test Client Specification
- JSON Body Validation Context
- Logger Specification
- Logger Tasks
- Metadata Registration Core Design
- Middleware Design
- Module Composition Specification
- Module Lazy Loading Specification
- Module Re-export Design
- Multipart Form Streaming Design
- Numeric & Boolean Branches Specification
- OpenAPI Document Builder Specification
- Panic Recovery & Default Handler Specification
- Param, Query & Custom Validation Context
- Param, Query & Custom Validation Design
- Param, Query & Custom Validation Specification
- Pipeline Ordering Specification
- Provider & DI Graph Design
- Provider & DI Graph Specification
- Provider Interface Export Specification
- Provider-side MustInjectAll Specification
- T1: Extract Request from Context
- Spec: Route-level MustInject
- Scheduler Specification
- Schema Generation Specification
- Schema Sanitize/Refine Specification
- String-family Branches Specification
- Swagger UI Setup Specification
- Terminus/Health Check Specification
- Test App Bootstrap Spec
- Unified Parse API Spec
- Unified Token (TokenRef) Spec
- AD-001: Fluxo de trabalho em 3 papéis por feature
- AD-003: Skills vendorizadas em .agents/skills
- AD-004: 1 pacote Go por tipo sob internal/
- AD-005: Transient sem consumidor nunca instancia
- AD-006: MustResolve renomeado pra MustInject
- AD-007: NewApp[T] genérico usa idiom de 2 type param
- AD-009: pacote raiz gonest consolidado
- AD-010: Renames de pacotes internal
- AD-014: Schema Generation mapeia @nestjs/swagger
- AD-050: gonest.XxxRef reexports
- AD-053: gonest.ProviderAs[T] explícito
- AD-054: Module.Lazy / gonest.LazyModule
- AD-055: Workflow Conventions formalizadas
- AD-056: TokenRef unifica markers de Module
- AD-057: Mensagem de panic de lifecycle hook
- AD-058: HttpException.Error() fallback JSON
- AD-059: banner de startup localhost:PORT
- AD-060: gonest.MustSetupSwagger
- AD-061: HttpContext unifica (req, res)
- Unified Parse API

## God Nodes (most connected - your core abstractions)
1. `New()` - 83 edges
2. `PropertyBuilder` - 74 edges
3. `ProviderRef` - 59 edges
4. `HttpException` - 58 edges
5. `Route` - 56 edges
6. `Context` - 51 edges
7. `New()` - 51 edges
8. `New()` - 49 edges
9. `Request` - 46 edges
10. `newBuiltin()` - 45 edges

## Surprising Connections (you probably didn't know these)
- `SyncAccessorFields()` --conceptually_related_to--> `Accessor Dirty-Tracking`  [INFERRED]
  gonest.go → README.md
- `MustParse()` --calls--> `gonest.Parseable`  [EXTRACTED]
  gonest.go → INSIGHT-GRPC.md
- `MustParse()` --calls--> `gonest.Schema`  [EXTRACTED]
  gonest.go → README.md
- `objectSchemaFor()` --references--> `ObjectSchema`  [EXTRACTED]
  .examples/full-text-search/person/dto.go → internal/schema/object.go
- `NewPersonProps()` --calls--> `NewAccessor()`  [INFERRED]
  .examples/full-text-search/shared/entity/entity.go → gonest.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Metadata Builder System** — internal_metadata, specs_features_metadata_registration_core_spec, specs_features_numeric_boolean_branches_design, specs_features_object_builder_design [EXTRACTED 0.90]
- **Module System Evolution** — specs_features_module_composition_spec, specs_features_module_lazy_loading_design, specs_features_module_reexport_spec [EXTRACTED 0.80]
- **Bootstrap & DI Evolution** — specs_project_state_archive_ad_015, specs_project_state_archive_ad_008, specs_project_state_archive_ad_006, specs_project_state_archive_ad_005 [INFERRED 0.80]
- **Metadata & Schema System** — specs_project_state_archive_ad_014, specs_project_state_archive_ad_013, specs_project_state_archive_ad_012, specs_project_state_archive_ad_011, specs_project_state_archive_ad_002, specs_project_state_archive_ad_049, specs_project_state_archive_ad_048 [INFERRED 0.90]
- **Module & Token Unification** — specs_project_state_archive_ad_056, specs_project_state_archive_ad_053, specs_project_state_archive_ad_052, specs_project_state_archive_ad_051, specs_project_state_archive_ad_050 [INFERRED 0.85]
- **Unified Transport Abstraction** — gonest_parseable, gonest_httpcontext, gonest_grpccontext, gonest_microservicecontext [EXTRACTED 1.00]
- **Core DI System** — gonest_module, gonest_provider, gonest_controller [EXTRACTED 1.00]
- **Milestone 19: Config Loading** — specs_features_dotenv_loading_tasks, specs_features_env_schema_binding_spec, specs_features_env_schema_binding_design, specs_features_env_schema_binding_tasks [EXTRACTED 1.00]
- **GraphQL Realtime Protocols & Support** — specs_features_graphql_support_spec, specs_features_graphql_realtime_protocols_spec, specs_features_graphql_realtime_protocols_design, specs_features_graphql_realtime_protocols_tasks [INFERRED 0.90]
- **Lifecycle Hooks Implementation Flow** — specs_features_lifecycle_hooks_tasks_t1, specs_features_lifecycle_hooks_tasks_t2, specs_features_lifecycle_hooks_tasks_t3, specs_features_lifecycle_hooks_tasks_t4, specs_features_lifecycle_hooks_tasks_t5, specs_features_lifecycle_hooks_tasks_t6, specs_features_lifecycle_hooks_tasks_t7 [EXTRACTED 1.00]

## Communities (318 total, 124 thin omitted)

### Community 0 - "Provider Lifecycle Testing"
Cohesion: 0.06
Nodes (65): callAndStore(), Provider, T, newResolvedProvider(), registerFuncs(), registerSignalFuncs(), TestHookRegistration_InvalidShapes(), TestHookRegistration_PanicMessage_NamesProviderTypeAndReceivedSignature() (+57 more)

### Community 1 - "inject_test.go"
Cohesion: 0.06
Nodes (73): allAsView, constructable, declarable, directResolver, fakePinger, fakeService, lazyConfig, lazyResolvedSetter (+65 more)

### Community 2 - "fiber_dispatch_test.go"
Cohesion: 0.07
Nodes (68): App, barFilterException, fooFilterException, graphqlUserEntity, nameParams, pipelineIDParams, UserEntity, userIDNameParams (+60 more)

### Community 3 - "parse_test.go"
Cohesion: 0.07
Nodes (61): Dotenv, envPair, envParseIntoFixture, Dotenv(), Get(), T, TestDotenv_ParseInto_DelegatesToParseEnvInto(), TestDotenv_SatisfiesParseable() (+53 more)

### Community 4 - "route_test.go"
Cohesion: 0.07
Nodes (50): New(), Reader, Schema, T, Type, Value, Writer, newFakeResponder() (+42 more)

### Community 5 - "validate.go"
Cohesion: 0.07
Nodes (48): ParseEnvInto(), T, TestParseEnvInto_AllFieldsSet_PopulatesCorrectly(), TestParseEnvInto_DefaultUsedWhenAbsent_RealValueUsedWhenPresent(), TestParseEnvInto_EmptyButSetEnvVar_TreatedAsPresent(), TestParseEnvInto_FieldWithoutEnvTag_Ignored(), NewGraphqlArgsSource(), normalizeGraphqlValue() (+40 more)

### Community 6 - "Event Emitter System"
Cohesion: 0.07
Nodes (37): barEvent, fooEvent, Listener, Listener[EventType], quxEvent, subscribeTestEvent, Emitter, Mutex (+29 more)

### Community 7 - "builtin.go"
Cohesion: 0.06
Nodes (53): BadGatewayException, ExpectationFailedException, FailedDependencyException, GatewayTimeoutException, GoneException, HTTPVersionNotSupportedException, InternalServerErrorException, MethodNotAllowedException (+45 more)

### Community 8 - "logger.go"
Cohesion: 0.09
Nodes (34): GetLogger(), GetLoggerFor(), TestMustNewApp_AppOptionsLogger_BecomesTheActiveFrameworkLogger(), Active(), Configure(), Debug(), defaultAllowed(), Error() (+26 more)

### Community 9 - "fakeResponder"
Cohesion: 0.07
Nodes (28): barExampleError, fakeResponder, Filter, fooExampleError, NewFilter(), Module, Type, Value (+20 more)

### Community 10 - "gonest_test.go"
Cohesion: 0.08
Nodes (48): Accessor, authService, idParams, insightListUsersQuery, insightPingable, insightPostgres, insightRedis, insightTestIUserService (+40 more)

### Community 11 - "New"
Cohesion: 0.13
Nodes (47): Generate(), schemaFor(), containsNull(), Schema, T, keys(), newUserSchema(), TestDocument_ProducesValidJSONMarshalableOutput() (+39 more)

### Community 12 - "validate_test.go"
Cohesion: 0.15
Nodes (45): TestParseEnvInto_IntegerCoercionFails_RecordsViolation(), TestParseEnvInto_TwoRequiredMissing_CollectsBothViolations(), expectBadRequest(), T, hasFieldViolation(), mustNotNullableBody(), newCtx(), nullableRequiredValidBody() (+37 more)

### Community 13 - "fakeResponder"
Cohesion: 0.07
Nodes (19): NewMiddleware(), Module, Type, Value, New(), Reader, T, Writer (+11 more)

### Community 14 - "fakeResponder"
Cohesion: 0.07
Nodes (19): NewInterceptor(), fakeResponder, Interceptor, Next, Module, Type, Value, New() (+11 more)

### Community 15 - "HttpContext"
Cohesion: 0.13
Nodes (32): HttpContext, reservation, ReservationRegistry, sseSingleFrame, sseSingleOperationBody, Module, registerGraphql(), NewHttpContext() (+24 more)

### Community 16 - "fakeResponder"
Cohesion: 0.07
Nodes (18): NewGuard(), fakeResponder, Guard, Module, Type, Value, New(), Reader (+10 more)

### Community 17 - "New"
Cohesion: 0.12
Nodes (37): ctrlAnimal, ctrlCat, ctrlFooService, New(), T, TestBearerAuth_SetsFlag(), TestController_SatisfiesModuleControllerRef(), TestController_SatisfiesModuleOwner() (+29 more)

### Community 18 - "Scheduler"
Cohesion: 0.10
Nodes (25): NewScheduler(), Duration, Module, Mutex, Once, Type, Value, New() (+17 more)

### Community 19 - "New"
Cohesion: 0.12
Nodes (36): consumerMarker, OnListen, New(), barFilterException, fooFilterException, nameParams, userIDNameParams, userIDParams (+28 more)

### Community 20 - "search.go"
Cohesion: 0.08
Nodes (30): collectFieldNames(), FieldNames(), FieldsSchemaFor(), Schema, T, Type, LikeMatch(), MatchField() (+22 more)

### Community 21 - "New"
Cohesion: 0.15
Nodes (37): New(), T, TestAssemble_AutoWiresOwnerModuleOnControllers(), TestAssemble_AutoWiresOwnerModuleOnProviders(), TestAssemble_DiamondImport_VisitsSharedModuleOnce(), TestAssemble_ExportDeclaredInProviders_NoError(), TestAssemble_ExportNotDeclaredInProviders_ReturnsError(), TestAssemble_SimpleBFS_VisitsImportedModule() (+29 more)

### Community 22 - "newArrayTestSchema"
Cohesion: 0.14
Nodes (36): addressEntity, Schema, T, newAddressTestSchema(), newArrayTestSchema(), TestArray_CalledTwice_ProducesIndependentItemState(), TestArray_SetsFormatAndReturnsNewArraySchema(), TestArraySchema_FieldMethods_NeverMutateItem() (+28 more)

### Community 23 - "httpFiberResponder"
Cohesion: 0.06
Nodes (5): Ctx, Reader, Writer, fakeResponder, httpFiberResponder

### Community 24 - "ws_protocol_test.go"
Cohesion: 0.19
Nodes (26): closeCall, orderCreatedEvent, protoFakeWSConn, T, TestWSProtocolAndSSEDistinct_SameSubscription_BothReceiveSameEmittedEvent(), ackConn(), Schema, T (+18 more)

### Community 25 - "direct_test.go"
Cohesion: 0.13
Nodes (29): FindDirect(), animalIfaceType(), fakeProvider, T, Type, Value, newResolved(), TestFindDirect_ImportedNotReExported_StillInvisible() (+21 more)

### Community 26 - "app.go"
Cohesion: 0.12
Nodes (29): declarable, pipelineStageType, routableController, asFilters(), asMiddleware(), composeHandler(), countTree(), declareControllers() (+21 more)

### Community 27 - "Module"
Cohesion: 0.09
Nodes (11): effectiveExports(), Module, TestModule_OwnControllers_ReturnsCopyNotInternalSlice(), TestModule_OwnResolvers_ReturnsCopyNotInternalSlice(), ControllerRef, FilterRef, ListenerRef, MiddlewareRef (+3 more)

### Community 28 - "PropertyBuilder"
Cohesion: 0.06
Nodes (4): addDescriptionAndExamples(), applyNullable(), Schema, PropertyBuilder

### Community 29 - "Route"
Cohesion: 0.08
Nodes (3): Schema, resolver, Route

### Community 30 - "Request"
Cohesion: 0.10
Nodes (9): BodySource, Parseable, Request, Responder, Schema, T, mustParse(), Reader (+1 more)

### Community 31 - "T"
Cohesion: 0.14
Nodes (30): Module, runApplicationBootstrapPhase(), runApplicationShutdownPhase(), runModuleDestroyPhase(), runModuleInitPhase(), App, Module, Provider (+22 more)

### Community 32 - "Bootstrap Order Tracking"
Cohesion: 0.12
Nodes (24): orderLog, tpProviderA, tpProviderB, tpProviderC, tpProviderX, tpProviderY, customScalarEntity, scalarEntity (+16 more)

### Community 34 - "fiber_test.go"
Cohesion: 0.13
Nodes (28): customTestException, HttpException, T, TestFiberWSConn_CloseWithCode_SendsRealCloseFrame(), TestInit_CalledTwice_DoesNotResetExistingApp(), TestInit_ZeroValueFiberApp_BecomesUsable(), TestListen_NilOnListen_DoesNotPanicAndBlocksNormally(), TestListen_OnListenFires_BeforeBlockingForGood() (+20 more)

### Community 35 - "New"
Cohesion: 0.17
Nodes (27): genEmailOnlyEntity, genPostEntity, genUserEntity, Build(), Mutation, Query, Schema, T (+19 more)

### Community 36 - "Resolve"
Cohesion: 0.13
Nodes (26): Resolve(), T, TestInvokeAndCopy_PendingAllEdge_WritesMatchedNodeIntoItsSlot(), TestResolve_ConstructorError_CancelsSiblingGoroutines(), TestResolve_ConstructorPanic_IsRecoveredAsError(), TestResolve_ConstructorReceivesConfiguredContext(), TestResolve_CopyInPlace_PlaceholderReflectsRealData(), TestResolve_DependentProvider_WaitsForDependencyDone() (+18 more)

### Community 37 - "New"
Cohesion: 0.23
Nodes (25): New(), T, TestReply_Html_DelegatesToResponder(), TestReply_Json_DelegatesToResponder(), TestReply_Request_ReturnsOriginatingRequest(), TestReply_SetHeader_WritesToResponder(), TestReply_Status_IsChainableAndSetsCode(), TestReply_Status_Json_Chained() (+17 more)

### Community 39 - "app/lifecycle_test.go"
Cohesion: 0.07
Nodes (26): abLeafType, abMxHookedType, abMxUnhookedType, abP1Type, abP2Type, abP3Type, abRootType, asLeafType (+18 more)

### Community 40 - "fakeResponder"
Cohesion: 0.08
Nodes (4): fakeResponder, WSConn, Reader, Writer

### Community 41 - "Subscription"
Cohesion: 0.11
Nodes (7): Resolver, Subscription, Module, Mutation, Query, Schema, newSubscription()

### Community 42 - "gonest.go"
Cohesion: 0.11
Nodes (25): Accessor Dirty-Tracking, Emitter, EventType, Schema, T, Value, loggerOutcome, MustInject() (+17 more)

### Community 43 - "graphqlPostDispatcher"
Cohesion: 0.11
Nodes (23): graphqlPostBodyPeek, graphqlRequestBody, graphqlResponseBody, resolvableResolver, FormFile, Schema, graphqlGetDispatcher(), graphqlHandler() (+15 more)

### Community 44 - "Context"
Cohesion: 0.19
Nodes (12): Context, describeGivenFunc(), Provider, Provider, Type, Value, invokeHook(), isValidHookSignature() (+4 more)

### Community 45 - "Find"
Cohesion: 0.20
Nodes (24): Find(), findExported(), findOwn(), Module, Type, hasOwnUnexported(), barType(), bazType() (+16 more)

### Community 46 - "must_inject_all_provider_test.go"
Cohesion: 0.10
Nodes (16): mustInjectAllCacheAdapter, mustInjectAllOrderingAdapterA, mustInjectAllOrderingAdapterB, mustInjectAllOrderingAdapterC, mustInjectAllOrderingLog, mustInjectAllPingable, mustInjectAllSQLAdapter, mustInjectAllTransientAdapter (+8 more)

### Community 47 - "NewModule"
Cohesion: 0.18
Nodes (25): Module, NewApp(), NewController(), NewModule(), TestApp_MustListen_NilOnListen_ThroughRootAlias(), TestApp_MustListen_PromotedThroughRootAlias(), TestAuthGuard_RootAlias_InsightCallShape(), TestFooExampleFilter_RootAlias_InsightCallShape() (+17 more)

### Community 49 - "HttpException"
Cohesion: 0.08
Nodes (15): fooExampleError, HttpException, NotAcceptableException, ProxyAuthRequiredException, RequestedRangeNotSatisfiableException, UpgradeRequiredException, NewHttpException(), NewNotAcceptableException() (+7 more)

### Community 50 - "Module"
Cohesion: 0.11
Nodes (8): Module, Type, TestProviderRef_ResolvedType_ExposesUnderlyingType(), fakeController, fakeListener, fakeProvider, fakeResolver, fakeScheduler

### Community 51 - "schema_test.go"
Cohesion: 0.20
Nodes (23): Schema, T, Time, newTestSchema(), TestNew_NonStructTypePanics(), TestOwnProperties_ReturnsAllRegisteredFields(), TestOwnProperties_ReturnsCopyNotInternalSlice(), TestProperty_DoesNotSwapNeighboringFields() (+15 more)

### Community 52 - "builtinCases"
Cohesion: 0.13
Nodes (22): BadRequestException, builtinCase, ConflictException, ForbiddenException, NotFoundException, UnauthorizedException, NewBadRequestException(), NewConflictException() (+14 more)

### Community 53 - "newInsightHealthModule"
Cohesion: 0.12
Nodes (14): Context, insightConnectable, insightConnectableService, insightHealthDb, insightHealthRedis, insightSchedulerUserService, Module, newInsightHealthModule() (+6 more)

### Community 54 - "direct_dispatch_test.go"
Cohesion: 0.16
Nodes (17): directIface, directImpl, fakeDirectOwner, Module, T, Type, Value, TestMustInject_DirectResolver_Interface_SingleMatch_Resolves() (+9 more)

### Community 55 - "openapi/generate.go"
Cohesion: 0.25
Nodes (21): buildResponses(), defaultErrorResponse(), defaultExceptionName(), formBodySchemaObject(), Module, OpenAPI, Schema, StructField (+13 more)

### Community 56 - "fakeSSEResponder"
Cohesion: 0.10
Nodes (5): fakeSSEResponder, Reader, Writer, PipeReader, PipeWriter

### Community 57 - "sync.go"
Cohesion: 0.22
Nodes (18): dirtyValue, PersonEntity, PersonProps, RawPerson, applyToDst(), collectDirtyAccessors(), collectDirtyAccessorsRecursive(), getAccessorValue() (+10 more)

### Community 58 - "NewHttpException"
Cohesion: 0.23
Nodes (17): Exception, ExceptionName(), EffectiveName(), NewHttpException(), T, TestEffectiveName_FallsBackToConcreteTypeName_WhenNameUnset(), TestEffectiveName_ReturnsSetNameWhenNonEmpty(), TestFooExampleError_SatisfiesException() (+9 more)

### Community 59 - "builder"
Cohesion: 0.25
Nodes (12): Field, FieldConfigArgument, FieldResolveFn, builder, argKey(), fieldResolver(), Schema, identityScalarConfig() (+4 more)

### Community 60 - "paramFakeResponder"
Cohesion: 0.10
Nodes (5): paramFakeResponder, Reader, TestParseRestFormBody_RealHTTPDispatch_StreamsFileWithoutFullBuffering(), Writer, WSConn

### Community 62 - "HttpAdapter"
Cohesion: 0.15
Nodes (14): HttpAdapter, httpAdapterPtr, TestBuilder, MustNewTestApp(), init(), T, Test, Module (+6 more)

### Community 63 - "provider_as_validate_test.go"
Cohesion: 0.16
Nodes (15): isProviderAsView, providerAsValidateAnimal, providerAsValidateCat, providerAsValidateDog, declareProviders(), Module, T, TestMustNewApp_ProviderAs_ConcreteMissing_Panics() (+7 more)

### Community 64 - "TestGenerateOpenApiSchema_RootAlias_InsightExample"
Cohesion: 0.16
Nodes (17): main(), main(), gonest.Controller, App, OpenAPI, gonest.HttpContext, gonest.Module, MustNewApp() (+9 more)

### Community 65 - "fiberResponder"
Cohesion: 0.11
Nodes (4): fiberResponder, Ctx, Reader, Writer

### Community 66 - "assemble"
Cohesion: 0.25
Nodes (16): assemble(), Module, moduleName(), validateExports(), T, TestAssemble_ExportModulesImported_NoError(), TestAssemble_ExportModulesNotImported_ReturnsError(), TestModule_EffectiveExports_Diamond_DedupesSharedReExportedModule() (+8 more)

### Community 67 - "stage3.go"
Cohesion: 0.26
Nodes (18): allProviders(), callConstructor(), edgesFor(), Module, Type, Value, invokeAndCopy(), invokeAndCopyEdge() (+10 more)

### Community 68 - "newNumericTestSchema"
Cohesion: 0.27
Nodes (18): Schema, T, newNumericTestSchema(), TestBoolean_CommonConstraintsWork(), TestBoolean_ReturnsSamePropertyBuilder(), TestBooleanThenInteger_NoPanicLastWins(), TestIntegerThenBoolean_NoPanicLastWins(), TestNumericFamilyBranches_CalledTwiceLastWins() (+10 more)

### Community 69 - "paramFakeResponder"
Cohesion: 0.11
Nodes (3): Reader, Writer, paramFakeResponder

### Community 70 - "queryFakeResponder"
Cohesion: 0.11
Nodes (3): Reader, Writer, queryFakeResponder

### Community 71 - "Schema"
Cohesion: 0.16
Nodes (5): cumulativeOffset(), findFieldByOffset(), Schema, StructField, Type

### Community 72 - "formFakeResponder"
Cohesion: 0.12
Nodes (3): Reader, Writer, formFakeResponder

### Community 73 - "params_test.go"
Cohesion: 0.22
Nodes (16): T, newParamCtx(), TestMustParams_CustomFunc_ReceivesRawString_NotCoerced(), TestMustParams_FieldWithNoRouteMatch_ProducesViolation(), TestMustParams_HappyPath_TwoParams(), TestMustParams_MismatchedSchema_PanicsBeforeReadingAnyParam(), TestMustParams_PresentButInvalid_ProducesViolation(), TestMustParams_RealHTTPDispatch_CustomFunc() (+8 more)

### Community 74 - "wiring_test.go"
Cohesion: 0.18
Nodes (11): controllableListenAdapter, wiringFailingBootstrapType, wiringFailingInitType, wiringHookedType, T, TestListen_AdapterListenError_ReturnsImmediatelyWithoutWaitingOnShutdownDone(), TestListen_ShutdownHooksDisabled_ReturnsAsSoonAsAdapterListenReturns(), TestListen_ShutdownHooksEnabled_BlocksUntilShutdownDoneThenReturnsShutdownErr() (+3 more)

### Community 75 - "Options"
Cohesion: 0.12
Nodes (7): fakeRegisteredRoute, listenSpyAdapter, Options, recordingFakeAdapter, newAdapter(), Mutex, PT

### Community 76 - "handleSubscribe"
Cohesion: 0.17
Nodes (13): wsProtocolMessage, wsSubscribePayload, NewGraphqlContext(), Execute(), Schema, T, newExecTestSchema(), TestExecute_InvalidQuery_ReturnsErrors() (+5 more)

### Community 77 - "NewBodySource"
Cohesion: 0.22
Nodes (15): NewBodySource(), T, newQueryCtx(), TestMustQuery_CustomFunc_ReceivesRawString_NotCoerced(), TestMustQuery_HappyPath_TwoParams(), TestMustQuery_MismatchedSchema_PanicsBeforeReadingAnyQuery(), TestMustQuery_MissingRequiredAndOutOfRange_BothCollected(), TestMustQuery_RealHTTPDispatch_CustomFunc() (+7 more)

### Community 78 - "app/lifecycle.go"
Cohesion: 0.16
Nodes (11): applicationBootstrapRunner, applicationShutdownRunner, beforeApplicationShutdownRunner, moduleDestroyRunner, moduleInitRunner, App, runBeforeApplicationShutdownPhase(), signalName() (+3 more)

### Community 79 - "Query"
Cohesion: 0.17
Nodes (5): Query, Mutation, newMutation(), Schema, newQuery()

### Community 80 - "SSEDistinctHandler"
Cohesion: 0.24
Nodes (13): Schema, SSEDistinctHandler(), streamSSEDistinctSubscription(), T, newSSEDistinctTestSchema(), TestSSEDistinctHandler_ClientDisconnects_HandlerGoroutineEnds(), TestSSEDistinctHandler_InvalidQuery_RespondsNextWithErrorNot400(), TestSSEDistinctHandler_Subscription_EmitsNextPerEmittedValue() (+5 more)

### Community 81 - "providerAsRef"
Cohesion: 0.15
Nodes (7): Module, Type, Value, ProviderAs(), hasResolvedValue, isProviderAsView, providerAsRef

### Community 83 - "NewRequestScope"
Cohesion: 0.22
Nodes (12): Mutex, NewRequestScope(), requestIDFrom(), T, TestNewRequestScope_ReturnsUsableCache(), TestRequestScope_DifferentContexts_ReturnDifferentInstances(), TestRequestScope_Get_NoRequestIDOnContext_ReportsNotFound(), TestRequestScope_SameContext_ReturnsSameInstance() (+4 more)

### Community 84 - "newStringTestSchema"
Cohesion: 0.32
Nodes (15): Schema, T, newStringTestSchema(), TestPropertyBuilder_FormatValueDefaultsEmpty(), TestStringFamilyBranches_CalledTwiceLastWins(), TestStringFamilyBranches_SetsCorrectFormat(), TestStringSchema_CommonConstraintsMutateSharedBuilderAndStayChainable(), TestStringSchema_EnumCalledTwiceLastWins() (+7 more)

### Community 85 - "validate/duration_test.go"
Cohesion: 0.22
Nodes (15): Duration, T, TestDuration_Env_Absent_UsesTypedDefault(), TestDuration_Env_Present_ParsesStringValue(), TestDuration_JSONBody_AboveMax_ProducesFieldViolation(), TestDuration_JSONBody_BelowMin_ProducesFieldViolation(), TestDuration_JSONBody_EnumAllowedValue_Populates(), TestDuration_JSONBody_EnumViolation() (+7 more)

### Community 86 - "New"
Cohesion: 0.28
Nodes (13): New(), T, TestApply_WritesOnlyWhenDirty(), TestMarshalJSON_EmitsInnerValueDirectly(), TestNew_WithArg_DirtyAndValueSet(), TestNew_WithoutArgs_NotDirty(), TestOnDirty_CalledOnlyWhenDirty(), TestSet_MarksDirtyAndStoresValue() (+5 more)

### Community 87 - "provider_as_test.go"
Cohesion: 0.21
Nodes (12): fakeProvider, T, Value, TestProviderAs_ChainingAnotherProviderAsView_Panics(), TestProviderAs_NonInterfaceT_Panics(), TestProviderAs_ReportsTAsResolvedType(), TestProviderAs_ResolvedValue_DelegatesToWrappedRef(), TestProviderAs_ResolvedValue_NoDelegate_ReturnsFalse() (+4 more)

### Community 88 - "ProviderRef"
Cohesion: 0.28
Nodes (13): cycleError(), DetectCycle(), T, TestDetectCycle_DirectCycle_ReturnsFullChain(), TestDetectCycle_DisconnectedAcyclicComponents_NoFalsePositive(), TestDetectCycle_IndirectCycle_ReturnsFullChainNotJustFoundCycle(), TestDetectCycle_NoCycle_ReturnsNil(), TestDetectCycle_SelfLoop_IsCaught() (+5 more)

### Community 89 - "BuildGraph"
Cohesion: 0.25
Nodes (12): BuildGraph(), Module, T, resetForGraphTest(), TestBuildGraph_ExcludesControllerOwnedEdges(), TestBuildGraph_IncludesPendingAllEdges_AlongsidePendingEdges(), TestBuildGraph_NodeWithNoDependenciesHasEmptyList(), TestBuildGraph_SingleDependencyEdge() (+4 more)

### Community 90 - "newTestValueSchema"
Cohesion: 0.21
Nodes (13): Schema, Type, NewValue(), Schema, T, Type, newTestValueSchema(), TestIsValue_FalseForStructShapedSchema() (+5 more)

### Community 91 - "Accessor"
Cohesion: 0.20
Nodes (11): Accessor, objectSchemaFor(), Time, BodyCreateDTO, BodyUpdateDTO, ParamsDTO, QueryDTOWhere, MatchBool (+3 more)

### Community 92 - "lifecycle_e2e_test.go"
Cohesion: 0.27
Nodes (10): e2eLifecycleRecorder, e2eLifecycleType, e2eLifecycleTypeNoHooks, assertLog(), Mutex, Provider, T, newE2ELifecycleProvider() (+2 more)

### Community 93 - "entity/entity.go"
Cohesion: 0.29
Nodes (12): Creatable, Deletable, Indexable, PersonProps, Updatable, Time, NewCreatable(), NewDeletable() (+4 more)

### Community 94 - "Person"
Cohesion: 0.33
Nodes (8): Person, applySort(), Mutex, matchWhere(), paginationBounds(), sortLess(), Service, QueryDTO

### Community 95 - "TodoService"
Cohesion: 0.20
Nodes (5): Mutex, TodoEntity, TodoService, TodoStats, TodoStatsUsecase

### Community 97 - "form_test.go"
Cohesion: 0.35
Nodes (13): buildMultipartBody(), Buffer, T, newFormCtx(), TestMustFormBody_PanicsOnError(), TestParseFormBody_CustomFunc_ReceivesRawString_NotCoerced(), TestParseFormBody_HappyPath_FieldAndFile(), TestParseFormBody_MalformedMultipartBody_ReturnsOneViolation() (+5 more)

### Community 98 - "NewProvider"
Cohesion: 0.18
Nodes (9): Emitter, Provider, insightEmitterUserService, insightLoggerService, NewProvider(), routeMustInjectUsecase, TestEmitter_RootAlias_InsightUserCreatedExample(), TestGetLoggerFor_InsideProviderConstructor_ResolvesWithoutMustInject() (+1 more)

### Community 99 - "Lookup"
Cohesion: 0.24
Nodes (11): SchemaFor(), Schema, Type, Lookup(), Register(), T, TestLookup_NeverRegisteredType_ReturnsFalseNoPanic(), TestNew_CalledTwiceForSameType_Panics() (+3 more)

### Community 100 - "FindDirectAll"
Cohesion: 0.27
Nodes (10): Type, Value, candidateProviders(), FindDirectAll(), findDirectMatches(), Module, Type, Value (+2 more)

### Community 101 - "newDurationTestSchema"
Cohesion: 0.44
Nodes (12): Duration, Schema, T, newDurationTestSchema(), TestDuration_SetsCorrectFormatAndKind(), TestDurationSchema_CommonConstraintsMutateSharedBuilderAndStayChainable(), TestDurationSchema_EnumChainAndRoundTrip(), TestDurationSchema_EnumDefaultUnset() (+4 more)

### Community 102 - "New"
Cohesion: 0.37
Nodes (12): Schema, T, Time, newKindAddressTestSchema(), newKindTestSchema(), TestKindValue_ArrayItemBranches_MirrorPropertyBuilder(), TestKindValue_BooleanAndString_AreDifferent(), TestKindValue_EveryBranch() (+4 more)

### Community 104 - "openapi_test.go"
Cohesion: 0.30
Nodes (11): T, TestBearerAuth_SetsFlag(), TestContact_SetsAndOverwrites(), TestDescription_SetsAndOverwrites(), TestLicense_SetsAndOverwrites(), TestNew_DefaultValues(), TestNew_InsightBootstrapExample(), TestNew_NilFn_DoesNotPanic() (+3 more)

### Community 105 - "renderSwaggerUIHTML"
Cohesion: 0.30
Nodes (10): App, OpenAPI, renderSwaggerUIHTML(), SetupSwagger(), T, TestRenderSwaggerUIHTML_DifferentOptions_ProduceDifferentOutput(), TestRenderSwaggerUIHTML_InterpolatesOptions(), TestSetupSwagger_EmptyDocument_StillReturnsValidJSON() (+2 more)

### Community 106 - "newDateTimeTestSchema"
Cohesion: 0.36
Nodes (11): Schema, T, Time, newDateTimeTestSchema(), TestDate_ReturnsSamePropertyBuilder(), TestDateThenDateTime_NoPanicLastWins(), TestDateTime_CommonConstraintsWork(), TestDateTime_InsightCreatedAtUpdatedAtDeletedAtChains() (+3 more)

### Community 108 - "enum_test.go"
Cohesion: 0.35
Nodes (11): enumFixtureBody(), T, TestMustJsonBody_EnumAndPatternAndMin_AllViolationsCollected(), TestMustJsonBody_IntegerEnum_AllowedValue_Passes(), TestMustJsonBody_IntegerEnum_DisallowedValue_RecordsOneViolation(), TestMustJsonBody_NoEnumCall_AnyValueOfRightTypeStillPasses(), TestMustJsonBody_NullableEnumField_ExplicitNull_Accepted(), TestMustJsonBody_StringEnum_AllowedValue_Passes() (+3 more)

### Community 109 - "validate/testhelpers_test.go"
Cohesion: 0.41
Nodes (11): Schema, T, mustParseForm(), mustParseHeaders(), mustParseJSON(), mustParseParams(), mustParseQuery(), parseHeaders() (+3 more)

### Community 110 - "App"
Cohesion: 0.22
Nodes (4): App, fiberMethod(), New(), TestFiberMethod_UnknownHttpMethod_Panics()

### Community 111 - "newSanitizeRefineTestSchema"
Cohesion: 0.42
Nodes (10): Schema, T, newSanitizeRefineTestSchema(), TestPropertyBuilder_Sanitize_LastCallWins(), TestPropertyBuilder_Sanitize_StoresFn_RetrievableViaSanitizeFunc(), TestPropertyBuilder_SanitizeFunc_NeverCalled_ReturnsFalse(), TestSchema_OwnRefines_EmptyByDefault(), TestSchema_OwnRefines_ReturnsCopyNotInternalSlice() (+2 more)

### Community 112 - "validate/custom_test.go"
Cohesion: 0.33
Nodes (10): decodeV1Code(), T, mustMarshal(), TestCustomFunc_SameDefinitionReused_ProducesSameResult(), TestMustJsonBody_CustomFunc_DecodesCustomFormat_EndToEnd(), TestMustJsonBody_CustomFunc_ReturningError_ProducesViolation_CollectedWithOthers(), TestMustJsonBody_CustomFunc_WrongGoType_ProducesViolation_NeverPanics(), TestMustJsonBody_FieldWithoutCustom_PopulatesExactlyAsBefore() (+2 more)

### Community 115 - "Service"
Cohesion: 0.29
Nodes (5): PostCreatedEvent, PostEntity, Service, Emitter, Mutex

### Community 116 - "Service"
Cohesion: 0.27
Nodes (5): DB, Entity, NewDuplicateEmailException(), DuplicateEmailException, Service

### Community 117 - "FormFile"
Cohesion: 0.24
Nodes (5): FormFile, Reader, NewFormFile(), parseForm(), Part

### Community 118 - "module/lazy_test.go"
Cohesion: 0.36
Nodes (9): T, TestLazyModule_Exports_LandsOnOwnerModule(), TestLazyModule_Exports_ModuleRef_LandsOnOwnerModule(), TestLazyModule_Imports_LandsOnOwnerModule(), TestLazyModule_OwnProviders_DelegatesToOwner(), TestLazyModule_OwnProviders_IsDefensiveCopy(), TestModule_Lazy_NilFnIsNoOp(), TestModule_Lazy_RunsBeforeAssembleReadsImportsExports() (+1 more)

### Community 119 - "TestResponse"
Cohesion: 0.31
Nodes (5): TestResponse, Test, T, lookupJSONPath(), normalizeJSONValue()

### Community 121 - "newDefaultTestSchema"
Cohesion: 0.47
Nodes (8): Schema, T, newDefaultTestSchema(), TestPropertyBuilder_Default_LastCallWins(), TestPropertyBuilder_Default_ReturnsSelfForChaining(), TestPropertyBuilder_Default_SetsDefaultValue(), TestPropertyBuilder_DefaultValue_NeverCalled_ReturnsFalse(), defaultEntity

### Community 122 - "buildLazyDrivenApp"
Cohesion: 0.46
Nodes (7): lazyDriverConfig, buildLazyDrivenApp(), App, T, TestLazyModule_ConfigProviderConstructorRunsExactlyOnce(), TestLazyModule_PicksModuleA_RealHttpDispatch(), TestLazyModule_PicksModuleB_RealHttpDispatch()

### Community 125 - ".Get"
Cohesion: 0.32
Nodes (6): insightTestUserEntity, insightTestUserService, insightTestUserServiceMock, TestMustNewTestApp_NoOverride_DirectMustInject_UnitStyle(), TestSetupSwagger_RootAlias_InsightBootstrapCallShape(), TestSyncAccessorFields_RootExport()

### Community 126 - "UserEntity"
Cohesion: 0.50
Nodes (3): UserEntity, UserService, TestNewApp_UserProviderExample_ResolvesUsableUserService()

### Community 127 - "method_test.go"
Cohesion: 0.43
Nodes (7): T, TestHttpDelete_String(), TestHttpGet_String(), TestHttpMethod_String_Unknown(), TestHttpPost_String(), TestHttpPut_String(), TestHttpQuery_String()

### Community 129 - "newCustomTestSchema"
Cohesion: 0.50
Nodes (7): Schema, T, newCustomTestSchema(), TestPropertyBuilder_Custom_LastCallWins(), TestPropertyBuilder_Custom_StoresFn_RetrievableViaCustomFunc(), TestPropertyBuilder_CustomFunc_NeverCalled_ReturnsFalse(), customEntity

### Community 130 - "lefthook.yaml"
Cohesion: 0.29
Nodes (7): build, default, examples, fmt, test, test:race, vet

### Community 131 - "gonest.dev/gonest"
Cohesion: 0.25
Nodes (8): blog-api, blog-graphql, config-dotenv, full-text-search, gonest.dev/gonest, lifecycle-hooks, notification-driver, simple-todo

### Community 132 - "Subscribe"
Cohesion: 0.38
Nodes (4): Emitter, T, Type, Subscribe()

### Community 133 - "Service"
Cohesion: 0.43
Nodes (3): DB, Entity, Service

### Community 134 - "NewParamsSource"
Cohesion: 0.43
Nodes (7): newParamFakeResponder(), TestMustParams_RootPackage_HappyPath(), TestMustParams_RootPackage_PanicsOnConversionFailure(), TestMustParams_RootPackage_PanicsWhenParamNotDeclaredOnRoute(), TestParseRestParams_RootPackage_ReturnsErrorInsteadOfPanicking(), Parseable, NewParamsSource()

### Community 136 - "newGraphqlScalarTestSchema"
Cohesion: 0.52
Nodes (6): Schema, T, newGraphqlScalarTestSchema(), TestPropertyBuilder_GraphqlScalar_StoresName_RetrievableViaGraphqlScalarValue(), TestPropertyBuilder_GraphqlScalarValue_NeverCalled_ReturnsFalse(), graphqlScalarEntity

### Community 137 - "T5: internal/app shutdown orchestrator"
Cohesion: 0.38
Nodes (7): T1: Provider no-signal lifecycle hooks, T2: Provider signal lifecycle hooks, T3: HttpAdapter.Shutdown + FiberApp.Shutdown, T4: internal/app bootstrap-time phase runners, T5: internal/app shutdown orchestrator, T6: Wire lifecycle hooks into NewApp/Listen, T7: End-to-end lifecycle test

### Community 138 - "must_inject_inside_constructor_test.go"
Cohesion: 0.40
Nodes (5): mustInjectInsideConstructorConsumer, mustInjectInsideConstructorDep, T, TestMustInject_CalledInsideConstructor_ProducesResolveError(), TestMustNewApp_CalledInsideConstructor_Panics()

### Community 139 - "Service"
Cohesion: 0.40
Nodes (3): Service, DB, Entity

### Community 140 - "LockedException"
Cohesion: 0.27
Nodes (6): LockedException, ServiceUnavailableException, NewLockedException(), NewServiceUnavailableException(), NewLockedException(), NewServiceUnavailableException()

### Community 141 - "NewFooExampleError"
Cohesion: 0.33
Nodes (5): FooExampleError, HttpException, NewFooExampleError(), TestHttpException_RootAlias_SatisfiesException(), TestNewFilter_RootAlias_TypeCheck()

### Community 142 - "JSON Body Validation Tasks"
Cohesion: 0.33
Nodes (6): internal/execution, internal/metadata, JSON Body Validation Specification, JSON Body Validation Tasks, Metadata Registration Core Specification, Metadata Registration Core Tasks

### Community 143 - "reservation_test.go"
Cohesion: 0.53
Nodes (5): T, TestReservationRegistry_AttachThenRoute_ReturnsWriteFunc(), TestReservationRegistry_ConcurrentReserveAttachRoute_NoRace(), TestReservationRegistry_Reserve_ReturnsUniqueToken(), TestReservationRegistry_RouteBeforeAttach_ReturnsNotOk()

### Community 144 - "post/dto.go"
Cohesion: 0.40
Nodes (4): CreateBodyDTO, ListQueryDTO, ParamsDTO, UploadAttachmentFormDTO

### Community 145 - "newSchema"
Cohesion: 0.40
Nodes (4): schemaShape, Schema, T, newSchema()

### Community 146 - "gonest.Parseable"
Cohesion: 0.40
Nodes (5): gonest.Consumer, gonest.GrpcContext, gonest.GrpcService, gonest.MicroserviceContext, gonest.Parseable

### Community 147 - "options_test.go"
Cohesion: 0.60
Nodes (4): T, TestAppOptions_ZeroValue(), TestOnListen_Invocable(), TestOnListen_NilSafe()

### Community 149 - "simple-todo/controller.go"
Cohesion: 0.50
Nodes (3): createTodoBody, todoIDParams, updateTodoBody

### Community 157 - "InsufficientStorageException"
Cohesion: 0.67
Nodes (3): InsufficientStorageException, NewInsufficientStorageException(), NewInsufficientStorageException()

### Community 158 - "LengthRequiredException"
Cohesion: 0.67
Nodes (3): LengthRequiredException, NewLengthRequiredException(), NewLengthRequiredException()

### Community 159 - "LoopDetectedException"
Cohesion: 0.67
Nodes (3): LoopDetectedException, NewLoopDetectedException(), NewLoopDetectedException()

### Community 160 - "MisdirectedRequestException"
Cohesion: 0.67
Nodes (3): MisdirectedRequestException, NewMisdirectedRequestException(), NewMisdirectedRequestException()

### Community 161 - "NotExtendedException"
Cohesion: 0.67
Nodes (3): NotExtendedException, NewNotExtendedException(), NewNotExtendedException()

### Community 162 - "PreconditionRequiredException"
Cohesion: 0.67
Nodes (3): PreconditionRequiredException, NewPreconditionRequiredException(), NewPreconditionRequiredException()

### Community 163 - "RequestHeaderFieldsTooLargeException"
Cohesion: 0.67
Nodes (3): RequestHeaderFieldsTooLargeException, NewRequestHeaderFieldsTooLargeException(), NewRequestHeaderFieldsTooLargeException()

### Community 164 - "RequestTimeoutException"
Cohesion: 0.67
Nodes (3): RequestTimeoutException, NewRequestTimeoutException(), NewRequestTimeoutException()

### Community 165 - "TooManyRequestsException"
Cohesion: 0.67
Nodes (3): TooManyRequestsException, NewTooManyRequestsException(), NewTooManyRequestsException()

### Community 166 - "UnavailableForLegalReasonsException"
Cohesion: 0.67
Nodes (3): UnavailableForLegalReasonsException, NewUnavailableForLegalReasonsException(), NewUnavailableForLegalReasonsException()

### Community 167 - "UnprocessableEntityException"
Cohesion: 0.67
Nodes (3): UnprocessableEntityException, NewUnprocessableEntityException(), NewUnprocessableEntityException()

### Community 168 - "UnsupportedMediaTypeException"
Cohesion: 0.67
Nodes (3): UnsupportedMediaTypeException, NewUnsupportedMediaTypeException(), NewUnsupportedMediaTypeException()

### Community 169 - "jsonBodyUserEntity"
Cohesion: 0.67
Nodes (3): jsonBodyAddressEntity, jsonBodyUserEntity, Time

### Community 170 - "Response"
Cohesion: 0.67
Nodes (3): Response, Request/Response Split Specification, T2: Extract Response from Context

### Community 175 - "Filter Design"
Cohesion: 0.67
Nodes (3): Filter Design, Filter Specification, Filter Tasks

### Community 176 - "GraphQL Realtime Protocols Specification"
Cohesion: 0.67
Nodes (3): GraphQL Realtime Protocols Design, GraphQL Realtime Protocols Specification, GraphQL Realtime Protocols Tasks

### Community 177 - "Object Builder Tasks"
Cohesion: 0.67
Nodes (3): Object Builder Design, Object Builder Specification, Object Builder Tasks

## Knowledge Gaps
- **288 isolated node(s):** `blog-api`, `CreateBodyDTO`, `ListQueryDTO`, `Entity`, `CreateBodyDTO` (+283 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **124 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Deregister()` connect `New` to `Bootstrap Order Tracking`, `newCustomTestSchema`, `fiber_dispatch_test.go`, `Lookup`, `route_test.go`, `newDurationTestSchema`, `New`, `newNumericTestSchema`, `newGraphqlScalarTestSchema`, `validate.go`, `newDateTimeTestSchema`, `New`, `newSanitizeRefineTestSchema`, `schema_test.go`, `newStringTestSchema`, `newArrayTestSchema`, `newDefaultTestSchema`, `newTestValueSchema`?**
  _High betweenness centrality (0.119) - this node is a cross-community bridge._
- **Why does `Context` connect `Context` to `stage3.go`, `Resolve`, `Event Emitter System`, `Subscription`, `wiring_test.go`, `Options`, `handleSubscribe`, `app/lifecycle.go`, `App`, `Query`, `HttpMethod`, `NewRequestScope`, `Scheduler`, `ws_protocol_test.go`, `builder`, `Request`, `T`?**
  _High betweenness centrality (0.100) - this node is a cross-community bridge._
- **Why does `Request` connect `Request` to `fiberResponder`, `form_test.go`, `New`, `Reply`, `params_test.go`, `wiring_test.go`, `Options`, `graphqlPostDispatcher`, `NewBodySource`, `App`, `HttpContext`, `validate/testhelpers_test.go`, `validate_test.go`, `HttpMethod`, `FormFile`?**
  _High betweenness centrality (0.088) - this node is a cross-community bridge._
- **Are the 69 inferred relationships involving `New()` (e.g. with `registerGraphql()` and `runApplicationBootstrapPhase()`) actually correct?**
  _`New()` has 69 INFERRED edges - model-reasoned connections that need verification._
- **Are the 27 inferred relationships involving `ProviderRef` (e.g. with `TestFindAllRefs_DiamondImport_Dedups()` and `TestFindAllRefs_ImportExportedMatch()`) actually correct?**
  _`ProviderRef` has 27 INFERRED edges - model-reasoned connections that need verification._
- **What connects `blog-api`, `CreateBodyDTO`, `ListQueryDTO` to the rest of the system?**
  _288 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Provider Lifecycle Testing` be split into smaller, more focused modules?**
  _Cohesion score 0.05877167205406994 - nodes in this community are weakly interconnected._