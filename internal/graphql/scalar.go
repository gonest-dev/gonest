// scalar.go maps a PropertyBuilder's OpenAPI format (or explicit
// GraphqlScalar name) to the GraphQL custom scalar name generate.go's
// Build uses -- see the package doc comment in resolver.go.
package graphql

import (
	"fmt"

	"gonest.dev/gonest/internal/schema"
)

// nativeFormatScalarNames maps a PropertyBuilder's OpenAPI format string
// (PropertyBuilder.FormatValue()) to the GraphQL custom scalar name the
// SDL generator declares for it -- spec.md's GQL-03 ("branches de formato
// viram Custom Scalars"). Bare "string"/""(String()'s own no-format
// branch) and every numeric/boolean format have NO entry here -- those map
// to GraphQL's own built-in String/Int/Float/Boolean, not a custom scalar.
var nativeFormatScalarNames = map[string]string{
	"email":     "Email",
	"uuid":      "Uuid",
	"uri":       "Uri",
	"hostname":  "Hostname",
	"ipv4":      "Ipv4",
	"ipv6":      "Ipv6",
	"password":  "Password",
	"byte":      "Byte",
	"binary":    "Binary",
	"date-time": "DateTime",
	"date":      "Date",
	"duration":  "Duration",
}

// NativeScalarName returns the GraphQL custom scalar name generated for a
// native OpenAPI format (e.g. "email" -> "Email"), and whether one exists.
func NativeScalarName(format string) (string, bool) {
	name, ok := nativeFormatScalarNames[format]
	return name, ok
}

// CollectScalars walks properties, returning the DEDUPLICATED (by name,
// not pointer identity -- design.md's Tech Decisions: multiple DIFFERENT
// PropertyBuilders can share the same scalar) set of scalar names required
// across all of them, in first-seen order -- both native-format-derived
// (NativeScalarName) and explicitly named via GraphqlScalar (schema.
// PropertyBuilder.GraphqlScalar).
//
// Returns an error (spec.md AC4, a build-time configuration failure, never
// a per-request one) when a property has Custom(fn) set but NEITHER a
// native format NOR an explicit GraphqlScalar name -- the SDL generator
// has no way to know what scalar name to declare for it.
func CollectScalars(properties []*schema.PropertyBuilder) ([]string, error) {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}

	for _, p := range properties {
		if name, ok := NativeScalarName(p.FormatValue()); ok {
			add(name)
			continue
		}
		if name, ok := p.GraphqlScalarValue(); ok {
			add(name)
			continue
		}
		if _, isCustom := p.CustomFunc(); isCustom {
			return nil, fmt.Errorf("gonest: field %q uses Custom(fn) in a GraphQL-exposed schema without .GraphqlScalar(name) -- the SDL generator has no scalar name to declare for it", p.Field().Name)
		}
	}

	return names, nil
}
