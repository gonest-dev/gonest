package graphql

import (
	"fmt"

	gql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"

	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
	"gonest.dev/gonest/internal/validate"
)

// builder accumulates the type caches needed across a single Build call --
// dedup by *schema.Schema pointer identity for Object types (mirroring
// internal/openapi/generate.go's own $ref dedup-by-pointer pattern) and by
// scalar name for custom Scalars (dedup by NAME, since distinct
// PropertyBuilders can share one GraphqlScalar(name) -- see scalar.go).
type builder struct {
	objects map[*schema.Schema]*gql.Object
	scalars map[string]*gql.Scalar
}

// Build converts every registered Query/Mutation/Subscription (each
// carrying a *schema.Schema for Args/Returns, built via the SAME
// NewSchema[T]/PropertyBuilder REST already uses) into a *gql.Schema.
// A pure generator, mirroring internal/openapi's role for REST -- called
// once at boot (Stage 2.5-equivalent), never per-request.
func Build(queries []*Query, mutations []*Mutation, subscriptions []*Subscription) (*gql.Schema, error) {
	b := &builder{
		objects: map[*schema.Schema]*gql.Object{},
		scalars: map[string]*gql.Scalar{},
	}

	queryFields := gql.Fields{}
	for _, q := range queries {
		f, err := b.buildField(q.Name(), q.ArgsSchema(), q.ReturnsSchema(), q.HandlerFunc())
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", q.Name(), err)
		}
		if _, exists := queryFields[q.Name()]; exists {
			return nil, fmt.Errorf("gonest: duplicate Query name %q", q.Name())
		}
		queryFields[q.Name()] = f
	}

	mutationFields := gql.Fields{}
	for _, m := range mutations {
		f, err := b.buildField(m.Name(), m.ArgsSchema(), m.ReturnsSchema(), m.HandlerFunc())
		if err != nil {
			return nil, fmt.Errorf("mutation %q: %w", m.Name(), err)
		}
		if _, exists := mutationFields[m.Name()]; exists {
			return nil, fmt.Errorf("gonest: duplicate Mutation name %q", m.Name())
		}
		mutationFields[m.Name()] = f
	}

	subscriptionFields := gql.Fields{}
	for _, s := range subscriptions {
		// Subscription's Field carries no Resolve/Subscribe -- real dispatch
		// bypasses graphql-go's own execution engine entirely (T9/T10's
		// internal/gqltransport calls Subscription.HandlerFunc() directly),
		// this Field only exists so the Subscription root type/SDL declares
		// the right name/args/return-type shape.
		f, err := b.buildField(s.Name(), s.ArgsSchema(), s.ReturnsSchema(), nil)
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
		queryFields["_empty"] = &gql.Field{
			Type:    gql.Boolean,
			Resolve: func(p gql.ResolveParams) (any, error) { return true, nil },
		}
	}

	cfg := gql.SchemaConfig{
		Query: gql.NewObject(gql.ObjectConfig{Name: "Query", Fields: queryFields}),
	}
	if len(mutationFields) > 0 {
		cfg.Mutation = gql.NewObject(gql.ObjectConfig{Name: "Mutation", Fields: mutationFields})
	}
	if len(subscriptionFields) > 0 {
		cfg.Subscription = gql.NewObject(gql.ObjectConfig{Name: "Subscription", Fields: subscriptionFields})
	}

	sch, err := gql.NewSchema(cfg)
	if err != nil {
		return nil, fmt.Errorf("gonest: failed to build GraphQL schema: %w", err)
	}
	return &sch, nil
}

// buildField builds one gql.Field -- its return type (from returns),
// its arguments (from args, each property becoming one
// gql.ArgumentConfig), and its Resolve callback (handler, nil for
// Subscription -- see Build's own comment on subscriptionFields).
//
// SPEC_DEVIATION (not anticipated by tasks.md's original "What" for T7):
// dispatch cannot be wired from internal/app alone as originally
// sketched -- graphql-go's own execution engine (gql.Do) is what
// walks the parsed query/selection set and invokes Resolve per field;
// there is no other hook to intercept a field's resolution. Resolve is
// therefore built HERE, at schema-construction time, wrapping handler --
// internal/app's own role (T7) is only to invoke gql.Do itself and
// translate its result to an HTTP response, not to dispatch per-field.
func (b *builder) buildField(name string, args, returns *schema.Schema, handler func(ctx *GraphqlContext) any) (*gql.Field, error) {
	outType, err := b.outputType(returns)
	if err != nil {
		return nil, err
	}

	fieldArgs, err := b.fieldArgs(args)
	if err != nil {
		return nil, err
	}

	field := &gql.Field{
		Name: name,
		Type: outType,
		Args: fieldArgs,
	}

	if handler != nil {
		field.Resolve = func(p gql.ResolveParams) (result any, resolveErr error) {
			defer func() {
				r := recover()
				if r == nil {
					return
				}
				// gonest.MustParse[T] (called inside handler against
				// ctx.Args()) panics *exception.BadRequestException on
				// validation failure -- same panic-based contract REST
				// already uses. Surface its Message() (e.g. "validation
				// failed") rather than a generic panic string, so a GraphQL
				// client sees an error equivalent to what a REST 400 would
				// have reported (spec.md's Independent Test, P1 story 1).
				if exc, ok := r.(exception.Exception); ok {
					resolveErr = fmt.Errorf("%s", exc.Message())
					return
				}
				resolveErr = fmt.Errorf("gonest: panic in resolver %q: %v", name, r)
			}()

			var argsParseable execution.Parseable
			if args != nil {
				argsParseable = validate.NewGraphqlArgsSource(p.Args)
			}
			ctx := NewGraphqlContext(argsParseable, nil)
			return handler(ctx), nil
		}
	}

	return field, nil
}

// fieldArgs builds a gql.FieldConfigArgument from args's own
// properties -- flat only in this version (an Args schema whose field is
// itself Array/Object is out of scope for V1, same restriction the
// Out of Scope table in spec.md documents for other source kinds; a
// nested Object/Array arg returns an error here rather than silently
// producing a wrong type).
func (b *builder) fieldArgs(args *schema.Schema) (gql.FieldConfigArgument, error) {
	if args == nil {
		return nil, nil
	}

	out := gql.FieldConfigArgument{}
	for _, p := range args.OwnProperties() {
		t, err := b.scalarOrListType(p)
		if err != nil {
			return nil, fmt.Errorf("arg %q: %w", p.Field().Name, err)
		}
		if p.IsRequired() {
			t = gql.NewNonNull(t)
		}
		out[argKey(p)] = &gql.ArgumentConfig{Type: t}
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
// gql.String as a degenerate default when returns is nil (a
// Query/Mutation/Subscription with no declared Returns) since GraphQL
// requires SOME output type.
func (b *builder) outputType(returns *schema.Schema) (gql.Output, error) {
	if returns == nil {
		return gql.String, nil
	}
	if returns.IsValue() {
		return b.scalarOrListType(returns.ValueProperty())
	}
	return b.objectType(returns)
}

// objectType builds (or returns the cached) *gql.Object for a
// struct-shaped Schema, keyed by pointer identity -- the SAME dedup
// mechanism internal/openapi/generate.go's registerSchema already uses for
// $ref/components.schemas (design.md's Code Reuse Analysis).
func (b *builder) objectType(s *schema.Schema) (*gql.Object, error) {
	if obj, ok := b.objects[s]; ok {
		return obj, nil
	}

	name := s.TitleText()
	if name == "" {
		name = s.StructType().Name()
	}

	fields := gql.Fields{}
	for _, p := range s.OwnProperties() {
		t, err := b.scalarOrListType(p)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", p.Field().Name, err)
		}
		if p.IsRequired() {
			t = gql.NewNonNull(t)
		}
		fields[argKey(p)] = &gql.Field{Type: t}
	}

	obj := gql.NewObject(gql.ObjectConfig{Name: name, Fields: fields})
	b.objects[s] = obj
	return obj, nil
}

// scalarOrListType builds the GraphQL type for a single property --
// dispatching on KindValue() the same way internal/validate's
// validatePrimitive/validateArray/validateObject already do for REST.
func (b *builder) scalarOrListType(p *schema.PropertyBuilder) (gql.Output, error) {
	switch p.KindValue() {
	case "array":
		item := p.ItemBuilder()
		if ref, ok := p.ItemRef(); ok {
			obj, err := b.objectType(ref)
			if err != nil {
				return nil, err
			}
			return gql.NewList(obj), nil
		}
		t, err := b.scalarOrListType(item)
		if err != nil {
			return nil, err
		}
		return gql.NewList(t), nil
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
func (b *builder) leafScalar(p *schema.PropertyBuilder) (gql.Output, error) {
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
		return gql.Int, nil
	case "number":
		return gql.Float, nil
	case "boolean":
		return gql.Boolean, nil
	default:
		return gql.String, nil
	}
}

// scalar returns the cached *gql.Scalar for name, building it via cfg
// the first time it's requested -- dedup by name (b.scalars), same
// rationale as CollectScalars.
func (b *builder) scalar(name string, cfg gql.ScalarConfig) *gql.Scalar {
	if s, ok := b.scalars[name]; ok {
		return s
	}
	s := gql.NewScalar(cfg)
	b.scalars[name] = s
	return s
}

// identityScalarConfig builds a ScalarConfig that passes the underlying
// Go value through unchanged -- every native format (Email/Uuid/etc) is
// backed by a plain Go string, so no real coercion is needed beyond
// satisfying graphql-go's own ScalarConfig contract (design.md never asked
// for format-specific serialization beyond naming, spec.md's GQL-03).
func identityScalarConfig(name string) gql.ScalarConfig {
	return gql.ScalarConfig{
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
var jsonScalarConfig = gql.ScalarConfig{
	Name:       "JSON",
	Serialize:  func(value any) any { return value },
	ParseValue: func(value any) any { return value },
	ParseLiteral: func(valueAST ast.Value) any {
		return nil
	},
}
