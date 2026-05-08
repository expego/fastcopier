# Changelog

All notable changes to FastCopier will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

### Added
- `Merge` — PATCH semantics: copies only non-zero source fields into destination.
- `Inspect` / `InspectPlan` — builds and returns the copy plan for a type pair without executing it; useful for startup auditing and AI coding agents.
- `MustRegister` — pre-builds and validates the copy plan at startup, panics on failure.
- `MustRegisterWithFields` — same as `MustRegister` but restricted to named fields.
- `Clone[T]` — generic convenience wrapper: deep-copies a value without pre-declaring a destination.
- `Map[Src, Dst]` — generic helper for `[]Src → []Dst` slice conversion.
- `RegisterCopier[Dst, Src]` — registers a zero-reflection copy function for a type pair; `Copy` routes to it automatically.
- `fastcopier-gen` CLI tool (`cmd/fastcopier-gen`) — generates `RegisterCopier` calls from Go types via AST analysis; output bypasses reflection entirely.
- `WithFields(...string)` option — restricts a copy to named fields only.
- `WithSkipZeroFields(bool)` option — skips zero-value source fields (PATCH semantics).
- `WithTagName(string)` option — per-call struct tag key override.
- `WithCircularReferencePolicy` option — choose between error or silent truncation on pointer cycles.
- `WithCopyBetweenPtrAndValue`, `WithIgnoreNonCopyableTypes`, `WithGlobalCache` options.
- `SetDefaultTagName` — changes the global struct tag key (default: `"fastcopier"`).
- `ClearCache` — discards all cached copy plans (useful in tests).
- Struct tag support: `fastcopier:"field_name"` for renaming, `fastcopier:"-"` for skipping, `,required`, `,nilonzero` tag options.
- Embedded struct field promotion (anonymous fields promoted to parent level).
- Flat struct fast path: same-type all-scalar structs copied with a single `reflect.Value.Set`.
- Flat slice bulk copy: `[]scalar`, `[]string`, `[]FlatStruct` etc. copied with a single `reflect.Copy`.
- Slice capacity reuse: destination slice backing array reused when capacity is sufficient.
- 64-shard plan cache for concurrent-safe, low-contention plan lookup.
- `sync.Pool` for `Context` objects — eliminates per-call heap allocation on the no-options path.
- Circular reference detection via lazy `visited` map (allocates only when a pointer cycle is encountered).
- Struct→Map and Map→Struct copy support (requires string map keys).
- Type conversion between compatible numeric and string types.
- Pointer↔value copy (controlled by `WithCopyBetweenPtrAndValue`).
- Interface source and destination handling.
- Array source and destination handling (copies min(src.Len, dst.Len) elements).
- Structured errors: `*CopyError` with field-level context; compatible with `errors.Is` / `errors.As`.

### Performance (Apple M4, `-benchtime=3s`)
- Simple struct (5 scalar fields): **51 ns/op**, 0 allocs/op (with generated code)
- Nested struct (struct + slices): **132 ns/op**, 0 allocs/op (with generated code)
- Complex struct (nested + slice of structs + map): **386 ns/op**, 2 allocs/op (with generated code)
- Pure reflection path (no generated code): zero allocations for structs and slices
- Maps: unavoidable allocations per call (`reflect.MapIter` requirement)

---

[Unreleased]: https://github.com/expego/fastcopier/compare/master...HEAD
