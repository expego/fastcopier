# Contributing to FastCopier

Thank you for considering a contribution! This document describes how to set up the project locally, run tests, and submit changes.

---

## Project Structure

```
fastcopier/
├── fastcopier.go         # Public API: Copy, Clone, Map, Merge, Inspect, MustRegister
├── engine.go             # Plan cache (64-shard), isFlatType, resolvePlan
├── plans.go              # Leaf plans: assign, convert, skip, ptr, iface, deferred
├── plan_struct.go        # structPlan — struct→struct field-by-field copy
├── plan_slice.go         # slicePlan — slice/array copy with bulk-copy fast path
├── plan_map.go           # mapPlan — map→map copy
├── plan_struct_to_map.go # structToMapPlan — struct→map[string]V
├── plan_map_to_struct.go # mapToStructPlan — map[string]V→struct
├── fields.go             # Struct field parsing, tag application, embedded struct support
├── inspect.go            # Inspect / InspectPlan — audit copy plans without executing
├── register.go           # RegisterCopier — zero-reflection custom copy registration
├── errors.go             # Sentinel errors and CopyError
├── cmd/fastcopier-gen/   # Optional CLI code generator (separate Go module)
├── benchmarks/           # Competitor benchmarks (separate Go module)
└── internal/gentest/     # Types used to validate code generator output
```

---

## Requirements

- Go 1.21 or later
- No external dependencies (the core library is pure stdlib)

---

## Getting Started

```bash
git clone https://github.com/expego/fastcopier.git
cd fastcopier
go test ./...
```

---

## Running Tests

```bash
# All tests
go test ./...

# Verbose output
go test -v ./...

# With coverage
go test -cover ./...

# Race detector
go test -race ./...
```

## Running Benchmarks

The core library has a single registered-copier benchmark:

```bash
go test -bench=. -benchmem -benchtime=3s -run='^$'
```

For the full competitor comparison suite (requires a separate module):

```bash
cd benchmarks
go test -bench=. -benchmem -benchtime=3s -count=1
```

## Running go vet

```bash
go vet ./...
```

---

## Guidelines

### Code style

- Match the style of the surrounding code (no helper libraries beyond what's already imported).
- Zero-allocation hot paths are a first-class concern — avoid adding `interface{}` boxing or extra maps in the execution path.
- No comments should be added or removed unless the change specifically justifies it.

### Tests

- Every new feature or bug fix must be accompanied by a test.
- Tests that exercise the public API live in `*_test.go` files with `package fastcopier_test`.
- Internal/white-box tests live in files with `package fastcopier`.
- Example tests (`Example*`) serve as both documentation and correctness checks.
- Target: maintain or improve the current statement coverage (currently ~85%).

### Plans and the Plan interface

When adding support for a new type pair:

1. Implement the `Plan` interface (`Copy(dst, src reflect.Value, ctx *Context) error`).
2. Add an `init(ctx *Context, dstType, srcType reflect.Type) error` method to build the plan.
3. Register the new plan in `resolvePlan` inside `engine.go`.
4. Add the plan to `inspectPlan` in `inspect.go` so `Inspect` produces correct output.

### Performance

If your change touches the hot path (struct copy of the same type pair), run the benchmarks before and after and include the results in your PR description.

```bash
go test -bench=BenchmarkRegisteredCopier -benchmem -benchtime=3s -count=5
```

---

## Submitting a Pull Request

1. Fork the repository and create a branch from `master`.
2. Make your changes.
3. Run `go test ./...` and `go vet ./...` — both must pass.
4. Open a pull request with a clear description of what the change does and why.

---

## Reporting Bugs

Open an issue with:
- Go version (`go version`)
- A minimal reproducer (a failing test is ideal)
- Expected vs. actual behavior

---

## Code of Conduct

Be respectful and constructive. Contributions of all kinds are welcome.
