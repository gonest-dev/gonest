// Package version exposes gonest's own build-time version string, read
// from Go's module build info (populated by `go install`/`go build` when
// building a tagged, published module). Building from a local checkout
// (`go run .`, this repo's own tests, .examples/*) has no such tag to
// read, so it falls back to "(devel)" -- honest about the fact that no
// v0.{major}-{minor}.{release} tag has been cut yet (see STATE.md's
// Preferences: versioning convention decided, not yet applied).
package version

import "runtime/debug"

const fallback = "(devel)"

// String returns gonest's own build version, or "(devel)" if unavailable
// (no tagged module version -- e.g. building from a local checkout).
func String() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fallback
	}
	if info.Main.Version == "" || info.Main.Version == "(devel)" {
		return fallback
	}
	return info.Main.Version
}
