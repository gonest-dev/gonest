// gonest/swagger/decorators.go
package swagger

// APIOperation defines operation metadata
type APIOperation struct {
	Summary     string
	Description string
	Tags        []string
	OperationID string
}

// APIResponse defines response metadata
type APIResponse struct {
	StatusCode  string
	Description string
	Type        any
}

// APIProperty defines property metadata
type APIProperty struct {
	Description string
	Example     any
	Required    bool
	Format      string
}

// APISecurity defines security requirements
type APISecurity struct {
	Name   string
	Scopes []string
}

// Metadata keys for storing Swagger info
const (
	MetadataOperation = "swagger:operation"
	MetadataResponse  = "swagger:response"
	MetadataParam     = "swagger:param"
	MetadataSecurity  = "swagger:security"
	MetadataTag       = "swagger:tag"
)
