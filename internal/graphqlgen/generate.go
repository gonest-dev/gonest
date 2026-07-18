package graphqlgen

import (
	"fmt"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"

	"gonest.dev/gonest/internal/gqlresolver"
	"gonest.dev/gonest/internal/schema"
)

// builder accumulates the type caches needed across a single Build call --
// dedup by *schema.Schema pointer identity for Object types (mirroring
// internal/openapi/generate.go's own $ref dedup-by-pointer pattern) and by
// scalar name for custom Scalars (dedup by NAME, since distinct
// PropertyBuilders can share one GraphqlScalar(name) -- see scalar.go).
type builder struct {
	objects map[*schema.Schema]*graphql.Object
	scalars map[string]*graphql.Scalar
}

// Build converts every registered Query/Mutation/Subscription (each
// carrying a *schema.Schema for Args/Returns, built via the SAME
// NewSchema[T]/PropertyBuilder REST already uses) into a *graphql.Schema.
// A pure generator, mirroring internal/openapi's role for REST -- called
// once at boot (Stage 2.5-equivalent), never per-request.
func Build(queries []*gqlresolver.Query, mutations []*gqlresolver.Mutation, subscriptions []*gqlresolver.Subscription) (*graphql.Schema, error) {
	b := &builder{
		objects: map[*schema.Schema]*graphql.Object{},
		scalars: map[string]*graphql.Scalar{},
	}

	queryFields := graphql.Fields{}
	for _, q := range queries {
		f, err := b.buildField(q.Name(), q.ArgsSchema(), q.ReturnsSchema())
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", q.Name(), err)
		}
		if _, exists := queryFields[q.Name()]; exists {
			return nil, fmt.Errorf("gonest: duplicate Query name %q", q.Name())
		}
		queryFields[q.Name()] = f
	}

	mutationFields := graphql.Fields{}
	for _, m := range mutations {
		f, err := b.buildField(m.Name(), m.ArgsSchema(), m.ReturnsSchema())
		if err != nil {
			return nil, fmt.Errorf("mutation %q: %w", m.Name(), err)
		}
		if _, exists := mutationFields[m.Name()]; exists {
			return nil, fmt.Errorf("gonest: duplicate Mutation name %q", m.Name())
		}
		mutationFields[m.Name()] = f
	}

	subscriptionFields := graphql.Fields{}
	for _, s := range subscriptions {
		f, err := b.buildField(s.Name(), s.ArgsSchema(), s.ReturnsSchema())
		if err != nil {
			return nil, fmt.Errorf("subscription %q: %w", s.Name(), err)
		}
		if _, exists := subscriptionFields[s.Name()]; exists {
			return nil, fmt.Errorf("gonest: duplicate Subscription name %q", s.Name())
		}
		subscriptionFields[s.Name()] = f
	}

	// graphql-go requires a non-nil Query root type even for a
	// mutation/subscription-only app (GraphQL spec itself mandates a Query
	// root type exists) -- a placeholder field keeps such a schema valid
	// without forcing every caller to declare a throwaway Query.
	if len(queryFields) == 0 {
		queryFields["_empty"] = &graphql.Field{
			Type:    graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (any, error) { return true, nil },
		}
	}

	cfg := graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: queryFields}),
	}
	if len(mutationFields) > 0 {
		cfg.Mutation = graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: mutationFields})
	}
	if len(subscriptionFields) > 0 {
		cfg.Subscription = graphql.NewObject(graphql.ObjectConfig{Name: "Subscription", Fields: subscriptionFields})
	}

	sch, err := graphql.NewSchema(cfg)
	if err != nil {
		return nil, fmt.Errorf("gonest: failed to build GraphQL schema: %w", err)
	}
	return &sch, nil
}

// buildField builds one graphql.Field -- its return type (from returns)
// and its arguments (from args, each property becoming one
// graphql.ArgumentConfig). Resolve/Subscribe are intentionally left nil:
// Query/Mutation dispatch is wired by internal/app (T7) directly against
// gqlresolver.Query/Mutation's own HandlerFunc, not through graphql-go's
// own execution engine's Resolve callback -- see design.md's Architecture
// Overview (gonest owns dispatch, graphql-go only supplies the type
// system/SDL). Subscription dispatch is likewise handled entirely by
// internal/gqltransport (T9/T10), never through graphql-go's own
// Subscribe/execution loop (context.md's research: graphql-go's own
// subscription support never shipped a streaming engine).
func (b *builder) buildField(name string, args, returns *schema.Schema) (*graphql.Field, error) {
	outType, err := b.outputType(returns)
	if err != nil {
		return nil, err
	}

	fieldArgs, err := b.fieldArgs(args)
	if err != nil {
		return nil, err
	}

	return &graphql.Field{
		Name: name,
		Type: outType,
		Args: fieldArgs,
	}, nil
}

// fieldArgs builds a graphql.FieldConfigArgument from args's own
// properties -- flat only in this version (an Args schema whose field is
// itself Array/Object is out of scope for V1, same restriction the
// Out of Scope table in spec.md documents for other source kinds; a
// nested Object/Array arg returns an error here rather than silently
// producing a wrong type).
func (b *builder) fieldArgs(args *schema.Schema) (graphql.FieldConfigArgument, error) {
	if args == nil {
		return nil, nil
	}

	out := graphql.FieldConfigArgument{}
	for _, p := range args.OwnProperties() {
		t, err := b.scalarOrListType(p)
		if err != nil {
			return nil, fmt.Errorf("arg %q: %w", p.Field().Name, err)
		}
		if p.IsRequired() {
			t = graphql.NewNonNull(t)
		}
		out[argKey(p)] = &graphql.ArgumentConfig{Type: t}
	}
	return out, nil
}

// argKey resolves an argument's GraphQL field name from its json tag (the
// same tag Args/Returns schemas already carry for REST), falling back to
// the Go field name.
func argKey(p *schema.PropertyBuilder) string {
	tag := p.Field().Tag.Get("json")
	if tag == "" || tag == "-" {
		return p.Field().Name
	}
	for i, c := range tag {
		if c == ',' {
			return tag[:i]
		}
	}
	return tag
}

// outputType builds the GraphQL output type for a whole Schema -- an
// Object (struct-shaped, dedup'd by pointer identity in b.objects) or a
// bare scalar/list (Value-shaped, schema-value-support feature). Returns
// graphql.String as a degenerate default when returns is nil (a
// Query/Mutation/Subscription with no declared Returns) since GraphQL
// requires SOME output type.
func (b *builder) outputType(returns *schema.Schema) (graphql.Output, error) {
	if returns == nil {
		return graphql.String, nil
	}
	if returns.IsValue() {
		return b.scalarOrListType(returns.ValueProperty())
	}
	return b.objectType(returns)
}

// objectType builds (or returns the cached) *graphql.Object for a
// struct-shaped Schema, keyed by pointer identity -- the SAME dedup
// mechanism internal/openapi/generate.go's registerSchema already uses for
// $ref/components.schemas (design.md's Code Reuse Analysis).
func (b *builder) objectType(s *schema.Schema) (*graphql.Object, error) {
	if obj, ok := b.objects[s]; ok {
		return obj, nil
	}

	name := s.TitleText()
	if name == "" {
		name = s.StructType().Name()
	}

	fields := graphql.Fields{}
	for _, p := range s.OwnProperties() {
		t, err := b.scalarOrListType(p)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", p.Field().Name, err)
		}
		if p.IsRequired() {
			t = graphql.NewNonNull(t)
		}
		fields[argKey(p)] = &graphql.Field{Type: t}
	}

	obj := graphql.NewObject(graphql.ObjectConfig{Name: name, Fields: fields})
	b.objects[s] = obj
	return obj, nil
}

// scalarOrListType builds the GraphQL type for a single property --
// dispatching on KindValue() the same way internal/validate's
// validatePrimitive/validateArray/validateObject already do for REST.
func (b *builder) scalarOrListType(p *schema.PropertyBuilder) (graphql.Output, error) {
	switch p.KindValue() {
	case "array":
		item := p.ItemBuilder()
		if ref, ok := p.ItemRef(); ok {
			obj, err := b.objectType(ref)
			if err != nil {
				return nil, err
			}
			return graphql.NewList(obj), nil
		}
		t, err := b.scalarOrListType(item)
		if err != nil {
			return nil, err
		}
		return graphql.NewList(t), nil
	case "object":
		if ref, ok := p.SchemaRef(); ok {
			return b.objectType(ref)
		}
		// AdditionalProperties() (open object, no fixed shape) has no
		// direct GraphQL equivalent -- represented as the JSON scalar
		// (dedup'd like any other named scalar).
		return b.scalar("JSON", jsonScalarConfig), nil
	default:
		return b.leafScalar(p)
	}
}

// leafScalar builds the GraphQL type for a non-array/object property --
// native format (email/uuid/etc), explicit GraphqlScalar(name), or one of
// GraphQL's own built-ins (String/Int/Float/Boolean).
func (b *builder) leafScalar(p *schema.PropertyBuilder) (graphql.Output, error) {
	if name, ok := NativeScalarName(p.FormatValue()); ok {
		return b.scalar(name, identityScalarConfig(name)), nil
	}
	if name, ok := p.GraphqlScalarValue(); ok {
		return b.scalar(name, identityScalarConfig(name)), nil
	}
	if _, isCustom := p.CustomFunc(); isCustom {
		return nil, fmt.Errorf("gonest: field %q uses Custom(fn) without .GraphqlScalar(name)", p.Field().Name)
	}

	switch p.KindValue() {
	case "integer":
		return graphql.Int, nil
	case "number":
		return graphql.Float, nil
	case "boolean":
		return graphql.Boolean, nil
	default:
		return graphql.String, nil
	}
}

// scalar returns the cached *graphql.Scalar for name, building it via cfg
// the first time it's requested -- dedup by name (b.scalars), same
// rationale as CollectScalars.
func (b *builder) scalar(name string, cfg graphql.ScalarConfig) *graphql.Scalar {
	if s, ok := b.scalars[name]; ok {
		return s
	}
	s := graphql.NewScalar(cfg)
	b.scalars[name] = s
	return s
}

// identityScalarConfig builds a ScalarConfig that passes the underlying
// Go value through unchanged -- every native format (Email/Uuid/etc) is
// backed by a plain Go string, so no real coercion is needed beyond
// satisfying graphql-go's own ScalarConfig contract (design.md never asked
// for format-specific serialization beyond naming, spec.md's GQL-03).
func identityScalarConfig(name string) graphql.ScalarConfig {
	return graphql.ScalarConfig{
		Name:       name,
		Serialize:  func(value any) any { return value },
		ParseValue: func(value any) any { return value },
		ParseLiteral: func(valueAST ast.Value) any {
			if v, ok := valueAST.(*ast.StringValue); ok {
				return v.Value
			}
			return nil
		},
	}
}

// jsonScalarConfig backs AdditionalProperties() open objects -- no fixed
// shape to declare fields for, so the whole value passes through as-is.
var jsonScalarConfig = graphql.ScalarConfig{
	Name:       "JSON",
	Serialize:  func(value any) any { return value },
	ParseValue: func(value any) any { return value },
	ParseLiteral: func(valueAST ast.Value) any {
		return nil
	},
}
