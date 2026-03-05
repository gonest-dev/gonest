package swagger

// ApiOperation defines operation metadata
type ApiOperation struct {
	Summary     string
	Description string
	Tags        []string
	OperationID string
}

// ApiResponse defines response metadata
type ApiResponse struct {
	StatusCode  string
	Description string
	Type        any
}

// ApiProperty defines property metadata
type ApiProperty struct {
	Description string
	Example     any
	Required    bool
	Format      string
}

// ApiSecurity defines security requirements
type ApiSecurity struct {
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
