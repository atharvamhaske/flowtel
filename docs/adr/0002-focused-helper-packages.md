# ADR 0002: Focused Helper Packages

Status: accepted

## Decision

Runtime structs live in `internal/config`. Reusable value bounds live in
`internal/bounds`. We do not create a generic `utils` or `helpers` package.

## Trade-off

The user-facing need for shared helpers is preserved, but each helper has a
named responsibility. This avoids a package that collects unrelated functions
and keeps dependencies pointing toward small, testable abstractions.
