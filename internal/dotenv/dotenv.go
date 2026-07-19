// Package dotenv implements the framework's ".env" file loading and
// env-var-to-struct binding (ParseInto, satisfying execution.Parseable). It
// deliberately has ZERO dependency on any DI/framework package
// (internal/inject, internal/module, internal/provider, internal/resolver,
// internal/app) -- gonest.Dotenv() must be safely callable as the very first
// line of main(), before any Module/Provider/NewApp exists. It does import
// internal/validate (for ParseEnvInto) and internal/execution (for the
// Parseable compile-time guard), neither of which import dotenv back, so no
// cycle is introduced. See .specs/features/dotenv-loading/design.md's
// Architecture Overview.
package dotenv

import (
	"fmt"
	"os"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/validate"
)

// Dotenv is the single type gonest.Dotenv() returns. It owns Load/MustLoad
// (dotenv-loading feature) and ParseInto (env-schema-binding feature,
// satisfying execution.Parseable). It has no fields yet.
type Dotenv struct{}

// instance is the package-level singleton every Get() call returns -- there
// is exactly ONE Dotenv, framework-owned, matching gonest.Dotenv()'s call
// shape (gonest.Dotenv().Load(...), never gonest.NewDotenv().Load(...)).
var instance = &Dotenv{}

// Get returns the package-level Dotenv singleton. NOT a New()-style
// constructor -- callers never get a fresh instance.
func Get() *Dotenv {
	return instance
}

// Load reads each path in paths, in order, and applies its resolved
// key/value pairs to the process environment. A path that doesn't exist
// propagates os.ReadFile's own error, wrapped. Parsing of a file's raw
// content is delegated to parseFile, which returns an ordered []envPair with
// values already fully resolved (quotes stripped, interpolation/escapes/
// inline-comments applied).
//
// Precedence: for each path, in the order received, and for each envPair in
// that path (in order), os.Setenv is called ONLY IF os.LookupEnv reports the
// key as absent. This single rule implements BOTH halves of the precedence
// policy (design.md's "Open Edge Cases Resolved"):
//   - "first path wins": a path processed earlier already called os.Setenv,
//     so a later path's LookupEnv check for the same key finds it present
//     and skips it.
//   - "pre-existing process env always wins": a key already set in the
//     process environment before Load ever runs is present from the very
//     first LookupEnv check, so no file value ever overwrites it.
func (d *Dotenv) Load(paths ...string) error {
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("gonest: dotenv load %q: %w", path, err)
		}

		pairs, err := parseFile(raw)
		if err != nil {
			return err
		}

		for _, pair := range pairs {
			if _, ok := os.LookupEnv(pair.Key); ok {
				continue
			}
			if err := os.Setenv(pair.Key, pair.Value); err != nil {
				return fmt.Errorf("gonest: dotenv setenv %q from %q: %w", pair.Key, path, err)
			}
		}
	}

	return nil
}

// MustLoad calls Load and panics if it returns a non-nil error -- same
// "Must-prefixed panics" convention as MustParse/MustNewApp across the
// framework.
func (d *Dotenv) MustLoad(paths ...string) {
	if err := d.Load(paths...); err != nil {
		panic(err)
	}
}

// ParseInto satisfies execution.Parseable, letting gonest.Parse[T]/
// gonest.MustParse[T] accept gonest.Dotenv() as a source directly (e.g.
// gonest.MustParse[DatabaseConfig](gonest.Dotenv(), schema)), exactly like
// req.Params()/req.Query()/req.Headers()/req.Body().Json() already do for
// HTTP sources -- env-schema-binding feature (T11). 100% delegation: all the
// actual read/validate/populate logic lives in validate.ParseEnvInto (T10),
// reused here unchanged.
func (d *Dotenv) ParseInto(dst any, schema any) error {
	return validate.ParseEnvInto(dst, schema)
}

// compile-time guard: *Dotenv must satisfy execution.Parseable, so
// gonest.Dotenv() type-checks as a Parse[T]/MustParse[T] source at the call
// site instead of only at generic-instantiation time.
var _ execution.Parseable = (*Dotenv)(nil)
