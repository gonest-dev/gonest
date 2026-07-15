package openapi

import "testing"

// TestNew_DefaultValues proves the zero-value *OpenAPI produced by
// New (when fn is a no-op) reports empty strings/false for every field --
// spec.md's Edge Cases: "WHEN BearerAuth() is never called THEN
// HasBearerAuth() SHALL report false -- no auth scheme by default", extended
// here to every other field per the same "no setter called yet" precedent
// established throughout internal/schema's own tests.
func TestNew_DefaultValues(t *testing.T) {
	doc := New("3.1.0", nil)

	if doc == nil {
		t.Fatal("New() returned nil")
	}
	if got := doc.SpecVersion(); got != "3.1.0" {
		t.Fatalf("SpecVersion() = %q, want %q", got, "3.1.0")
	}
	if got := doc.TitleText(); got != "" {
		t.Fatalf("TitleText() = %q, want empty", got)
	}
	if got := doc.DescriptionText(); got != "" {
		t.Fatalf("DescriptionText() = %q, want empty", got)
	}
	if got := doc.VersionText(); got != "" {
		t.Fatalf("VersionText() = %q, want empty", got)
	}
	name, url, email := doc.ContactInfo()
	if name != "" || url != "" || email != "" {
		t.Fatalf("ContactInfo() = (%q, %q, %q), want all empty", name, url, email)
	}
	licName, licUrl := doc.LicenseInfo()
	if licName != "" || licUrl != "" {
		t.Fatalf("LicenseInfo() = (%q, %q), want all empty", licName, licUrl)
	}
	if doc.HasBearerAuth() {
		t.Fatal("HasBearerAuth() = true, want false before BearerAuth() is called")
	}
}

// TestNew_RunsFn_WithSamePointerIdentity proves New builds a *OpenAPI,
// runs fn against it, and returns the SAME pointer fn received -- spec.md
// OD-01 ("construct a *OpenAPI ... run fn(doc), and return doc").
func TestNew_RunsFn_WithSamePointerIdentity(t *testing.T) {
	var received *OpenAPI

	doc := New("3.1.0", func(b *OpenAPI) {
		received = b
	})

	if received == nil {
		t.Fatal("fn was never called")
	}
	if received != doc {
		t.Fatalf("fn received pointer %p, want same pointer as returned %p", received, doc)
	}
}

// TestNew_NilFn_DoesNotPanic proves fn is optional -- New(specVersion, nil)
// simply builds the zero-value document and returns it.
func TestNew_NilFn_DoesNotPanic(t *testing.T) {
	doc := New("3.1.0", nil)
	if doc == nil {
		t.Fatal("New() returned nil")
	}
}

// TestTitle_SetsAndOverwrites proves Title stores its argument, retrievable
// via TitleText, and that a second call overwrites the first (spec.md's
// Edge Cases: "last-write-wins, same precedent as every branch method
// throughout internal/schema").
func TestTitle_SetsAndOverwrites(t *testing.T) {
	doc := New("3.1.0", nil)

	ret := doc.Title("First")
	if ret != doc {
		t.Fatalf("Title() returned %p, want same pointer %p for chaining", ret, doc)
	}
	if got := doc.TitleText(); got != "First" {
		t.Fatalf("TitleText() = %q, want %q", got, "First")
	}

	doc.Title("Second")
	if got := doc.TitleText(); got != "Second" {
		t.Fatalf("TitleText() = %q, want %q after overwrite", got, "Second")
	}
}

// TestDescription_SetsAndOverwrites mirrors TestTitle_SetsAndOverwrites for
// Description/DescriptionText.
func TestDescription_SetsAndOverwrites(t *testing.T) {
	doc := New("3.1.0", nil)

	ret := doc.Description("First")
	if ret != doc {
		t.Fatalf("Description() returned %p, want same pointer %p for chaining", ret, doc)
	}
	if got := doc.DescriptionText(); got != "First" {
		t.Fatalf("DescriptionText() = %q, want %q", got, "First")
	}

	doc.Description("Second")
	if got := doc.DescriptionText(); got != "Second" {
		t.Fatalf("DescriptionText() = %q, want %q after overwrite", got, "Second")
	}
}

// TestVersion_SetsAndOverwrites mirrors TestTitle_SetsAndOverwrites for
// Version/VersionText -- the API's OWN version, distinct from specVersion
// passed to New (spec.md AC1).
func TestVersion_SetsAndOverwrites(t *testing.T) {
	doc := New("3.1.0", nil)

	ret := doc.Version("1.0.0")
	if ret != doc {
		t.Fatalf("Version() returned %p, want same pointer %p for chaining", ret, doc)
	}
	if got := doc.VersionText(); got != "1.0.0" {
		t.Fatalf("VersionText() = %q, want %q", got, "1.0.0")
	}

	doc.Version("2.0.0")
	if got := doc.VersionText(); got != "2.0.0" {
		t.Fatalf("VersionText() = %q, want %q after overwrite", got, "2.0.0")
	}

	// specVersion (New's own arg) must remain untouched by Version(s).
	if got := doc.SpecVersion(); got != "3.1.0" {
		t.Fatalf("SpecVersion() = %q, want unaffected %q", got, "3.1.0")
	}
}

// TestContact_SetsAndOverwrites proves Contact(name, url, email) stores all
// 3 values together, retrievable via ContactInfo, and a second call
// overwrites all 3 (spec.md OD-03).
func TestContact_SetsAndOverwrites(t *testing.T) {
	doc := New("3.1.0", nil)

	ret := doc.Contact("Alice", "https://alice.example", "alice@example.com")
	if ret != doc {
		t.Fatalf("Contact() returned %p, want same pointer %p for chaining", ret, doc)
	}
	name, url, email := doc.ContactInfo()
	if name != "Alice" || url != "https://alice.example" || email != "alice@example.com" {
		t.Fatalf("ContactInfo() = (%q, %q, %q), want (%q, %q, %q)",
			name, url, email, "Alice", "https://alice.example", "alice@example.com")
	}

	doc.Contact("Bob", "https://bob.example", "bob@example.com")
	name, url, email = doc.ContactInfo()
	if name != "Bob" || url != "https://bob.example" || email != "bob@example.com" {
		t.Fatalf("ContactInfo() after overwrite = (%q, %q, %q), want (%q, %q, %q)",
			name, url, email, "Bob", "https://bob.example", "bob@example.com")
	}
}

// TestLicense_SetsAndOverwrites proves License(name, url) stores both
// values together, retrievable via LicenseInfo, and a second call
// overwrites both (spec.md OD-04).
func TestLicense_SetsAndOverwrites(t *testing.T) {
	doc := New("3.1.0", nil)

	ret := doc.License("MIT", "https://opensource.org/licenses/MIT")
	if ret != doc {
		t.Fatalf("License() returned %p, want same pointer %p for chaining", ret, doc)
	}
	name, url := doc.LicenseInfo()
	if name != "MIT" || url != "https://opensource.org/licenses/MIT" {
		t.Fatalf("LicenseInfo() = (%q, %q), want (%q, %q)",
			name, url, "MIT", "https://opensource.org/licenses/MIT")
	}

	doc.License("Apache-2.0", "https://apache.org/licenses/LICENSE-2.0")
	name, url = doc.LicenseInfo()
	if name != "Apache-2.0" || url != "https://apache.org/licenses/LICENSE-2.0" {
		t.Fatalf("LicenseInfo() after overwrite = (%q, %q), want (%q, %q)",
			name, url, "Apache-2.0", "https://apache.org/licenses/LICENSE-2.0")
	}
}

// TestBearerAuth_SetsFlag proves BearerAuth marks HasBearerAuth() true
// (spec.md OD-05), and that it's idempotent under repeat calls.
func TestBearerAuth_SetsFlag(t *testing.T) {
	doc := New("3.1.0", nil)

	ret := doc.BearerAuth()
	if ret != doc {
		t.Fatalf("BearerAuth() returned %p, want same pointer %p for chaining", ret, doc)
	}
	if !doc.HasBearerAuth() {
		t.Fatal("HasBearerAuth() = false, want true after BearerAuth()")
	}

	doc.BearerAuth()
	if !doc.HasBearerAuth() {
		t.Fatal("HasBearerAuth() = false after second BearerAuth() call, want true")
	}
}

// TestNew_InsightBootstrapExample reproduces INSIGHT.md's own bootstrap
// example verbatim (all 6 builder calls) and asserts every getter returns
// exactly what was set, plus the OpenAPI spec version string passed to New
// itself -- spec.md's "Independent Test" for the P1 user story.
func TestNew_InsightBootstrapExample(t *testing.T) {
	doc := New("3.1.0", func(b *OpenAPI) {
		b.Title("Example API")
		b.Description("An example API")
		b.Version("1.2.3")
		b.Contact("Support Team", "https://example.com", "support@example.com")
		b.License("MIT", "https://opensource.org/licenses/MIT")
		b.BearerAuth()
	})

	if got := doc.SpecVersion(); got != "3.1.0" {
		t.Fatalf("SpecVersion() = %q, want %q", got, "3.1.0")
	}
	if got := doc.TitleText(); got != "Example API" {
		t.Fatalf("TitleText() = %q, want %q", got, "Example API")
	}
	if got := doc.DescriptionText(); got != "An example API" {
		t.Fatalf("DescriptionText() = %q, want %q", got, "An example API")
	}
	if got := doc.VersionText(); got != "1.2.3" {
		t.Fatalf("VersionText() = %q, want %q", got, "1.2.3")
	}
	name, url, email := doc.ContactInfo()
	if name != "Support Team" || url != "https://example.com" || email != "support@example.com" {
		t.Fatalf("ContactInfo() = (%q, %q, %q), want (%q, %q, %q)",
			name, url, email, "Support Team", "https://example.com", "support@example.com")
	}
	licName, licUrl := doc.LicenseInfo()
	if licName != "MIT" || licUrl != "https://opensource.org/licenses/MIT" {
		t.Fatalf("LicenseInfo() = (%q, %q), want (%q, %q)",
			licName, licUrl, "MIT", "https://opensource.org/licenses/MIT")
	}
	if !doc.HasBearerAuth() {
		t.Fatal("HasBearerAuth() = false, want true")
	}
}
