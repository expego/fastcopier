// Package gentest provides types used to validate the fastcopier-gen code generator.
// This package is intentionally simple so it can be used as generator input.
package gentest

// Simple is a flat struct (all scalar fields) — the generator should emit
// a direct-assignment function with zero heap allocations.
type Simple struct {
	Name   string
	Age    int
	Email  string
	Score  float64
	Active bool
}

// Nested contains a flat struct field, slices, and a pointer — exercises the
// generator's slice-flat, struct, and pointer code paths.
type Nested struct {
	ID      int
	Profile Simple
	Tags    []string
	Scores  []int
}

// Complex exercises the full range of code paths: nested struct, struct slice,
// flat map, and scalar fields.
type Complex struct {
	ID       int
	Name     string
	Nested   Nested
	Items    []Simple
	Metadata map[string]string
}

// UserEntity / UserDTO are a cross-type copy pair used to test -src/-dst mode.
type UserEntity struct {
	ID       int
	Name     string
	Email    string
	Password string // private — not in DTO
}

// UserDTO is the public-facing representation of UserEntity.
type UserDTO struct {
	ID    int
	Name  string
	Email string
}
