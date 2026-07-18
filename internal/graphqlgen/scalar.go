// Package graphqlgen generates a graphql-go/graphql *graphql.Schema from
// gonest's own Schema/PropertyBuilder declarations (graphql-support
// feature, Milestone 17) -- mirrors internal/openapi's role for REST, a
// pure generator with no request-time logic of its own.
package graphqlgen

import "gonest.dev/gonest/internal/schema"

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
}

// NativeScalarName returns the GraphQL custom scalar name generated for a
// native OpenAPI format (e.g. "email" -> "Email"), and whether one exists.
func NativeScalarName(format string) (string, bool) {
	name, ok := nativeFormatScalarNames[format]
	return name, ok
}

// CollectScalars walks properties, returning the DEDUPLICATED (by name,
// not pointer identity -- design.md's Tech Decisions: multiple DIFFERENT
// PropertyBuilders can share the same scalar) set of native-format-derived
// scalar names required across all of them, in first-seen order.
func CollectScalars(properties []*schema.PropertyBuilder) []string {
	seen := map[string]bool{}
	var names []string
	for _, p := range properties {
		name, ok := NativeScalarName(p.FormatValue())
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
