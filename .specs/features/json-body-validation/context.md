# JSON Body Validation Context

User decisions captured via `AskUserQuestion` before specifying (2026-07-14) -- see `.specs/project/STATE.md`'s Discuss trigger criteria (Milestone 6 is genuinely new domain: first feature that READS metadata for real, not just builds it).

## Decision 1: Required-field presence detection

**Chosen**: 2-pass unmarshal. `MustJsonBody[T]` unmarshals the raw body TWICE:
1. Into `map[string]any` (or equivalent) -- ONLY to know which JSON keys were actually present, keyed by the struct field's `json` tag
2. Into `T` itself, normal `encoding/json` unmarshal -- to get the typed value

`Required()` validation fails if the field's `json` tag key is ABSENT from pass 1's map, regardless of what pass 2 produced (so `{}` fails `Name Required`, but `{"name":""}` PASSES the presence check -- an empty string was still explicitly sent; whether empty-string-when-required is ALSO an error is a separate concern, not presence).

**Rejected alternatives**: zero-value-only check (can't distinguish absent from empty), pointer-required-only (would force every Required field to be `*T`, contradicting `UserEntity`'s own INSIGHT.md shape where `Id int64`/`Name string` are Required as bare types, not pointers).

## Decision 2: Error collection mode

**Chosen**: collect ALL validation errors across the whole body, return them together in `BadRequestException`'s `details` (a list of `{field, message}`-shaped entries, one per violation) -- same UX class as Nest's `class-validator` (client sees every problem in one round-trip, not one-at-a-time).

**Rejected**: fail-fast (panic on first violation found) -- simpler to implement but worse UX (client needs N round-trips to fix N problems).

## Decision 3: Nested Array/Object validation scope

**Chosen**: RECURSIVE from the start. `Array()`-typed fields validate EVERY item against the item's own branch constraints (format/Min/Max/Pattern via the item's `*PropertyBuilder`, or recurse into `ItemRef()`'s `*Metadata` if the item is `Object(ref)`-typed). `Object()`-typed fields recurse into `MetadataRef()`'s `*Metadata` (or, if `IsAdditionalProperties()`, skip structural validation of the open sub-schema -- no fixed shape to check against).

**Rejected**: primitives-only-for-now (Array/Object deferred to a later feature) -- user explicitly chose the larger scope over the simpler one, accepting more surface area to get right on the first pass in exchange for not needing a follow-up feature later.

**Impact on design**: the validator needs a genuinely recursive core (`validateValue(value any, propBuilder *PropertyBuilder) []violation`-shaped, not a flat per-field loop) since `Array`/`Object` fields recurse into either another `*Metadata` (struct-shaped) or repeat the same item-branch check per slice element.
