# FastCopier

[![Go Reference](https://pkg.go.dev/badge/github.com/expego/fastcopier.svg)](https://pkg.go.dev/github.com/expego/fastcopier)
[![Go Report Card](https://goreportcard.com/badge/github.com/expego/fastcopier)](https://goreportcard.com/report/github.com/expego/fastcopier)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Fast, safe Go deep-copy library for structs, slices, maps — **zero allocations on repeated calls, no shared-memory surprises**.

`fastcopier.Copy` is drop-in simple, benchmark-proven, and built for production DTO mapping, PATCH merging, and hot-path data transforms.

## Install

```bash
go get github.com/expego/fastcopier
```

## Why teams pick FastCopier

- Correct deep copy semantics (no accidental slice/map aliasing)
- Zero allocations for repeated struct/slice copies
- Cycle-safe pointer traversal with configurable policy
- `Inspect` + `MustRegister` for startup-time mapping validation
- Optional `RegisterCopier` / `fastcopier-gen` path for reflection-free hot paths

## Performance

<!-- BENCHMARK_RESULTS_START -->

FastCopier beats every reflection-based competitor in fair benchmarks across 7 libraries.  
Benchmarks run on AMD EPYC 9V74 80-Core Processor, go1.25.0, `-benchtime=3s`.

### Simple Struct (5 primitive fields)

| Library | ns/op | B/op | allocs/op | vs FastCopier |
|---------|------:|-----:|----------:|:-------------:|
| Manual (baseline) | 0.354 | 0 | 0 | 315.1× faster |
| **FastCopier (with gen)** | 112 | 0 | 0 | **—** |
| FastCopier (pure reflect) | 145 | 0 | 0 | 1.3× slower |
| FastCopier.Clone | 171 | 128 | 2 | 1.5× slower |
| huandu/go-clone | 166 | 128 | 2 | 1.5× slower |
| tiendc/go-deepcopy | 192 | 32 | 1 | 1.7× slower |
| jinzhu/copier | 2,845 | 496 | 18 | **25.5× slower** |
| go-viper/mapstructure | 150 | 176 | 3 | 1.3× slower |
| ulule/deepcopier | 6,324 | 5,760 | 64 | **56.7× slower** |
| encoding/json | 1,769 | 336 | 7 | **15.9× slower** |

### Nested Struct (struct + slices)

| Library | ns/op | B/op | allocs/op | vs FastCopier |
|---------|------:|-----:|----------:|:-------------:|
| Manual (baseline) | 56.8 | 96 | 2 | 2.1× faster |
| **FastCopier (with gen)** | 117 | 0 | 0 | **—** |
| FastCopier (pure reflect) | 247 | 0 | 0 | 2.1× slower |
| FastCopier.Clone | 256 | 320 | 4 | 2.2× slower |
| huandu/go-clone | 426 | 480 | 7 | 3.6× slower |
| tiendc/go-deepcopy | 553 | 176 | 5 | 4.7× slower |
| jinzhu/copier | 2,380 | 600 | 16 | **20.3× slower** |
| go-viper/mapstructure | 194 | 288 | 4 | 1.7× slower |
| ulule/deepcopier | 4,423 | 3,792 | 43 | **37.7× slower** |
| encoding/json | 3,692 | 608 | 13 | **31.5× slower** |

### Complex Struct (nested + slice of structs + map)

| Library | ns/op | B/op | allocs/op | vs FastCopier |
|---------|------:|-----:|----------:|:-------------:|
| Manual (baseline) | 314 | 568 | 5 | on-par |
| **FastCopier (with gen)** | 327 | 336 | 2 | **—** |
| FastCopier (pure reflect) | 716 | 96 | 6 | 2.2× slower |
| FastCopier.Clone | 563 | 920 | 7 | 1.7× slower |
| huandu/go-clone | 1,763 | 1,568 | 21 | **5.4× slower** |
| tiendc/go-deepcopy | 1,365 | 432 | 13 | 4.2× slower |
| jinzhu/copier | 2,941 | 720 | 18 | **9.0× slower** |
| go-viper/mapstructure | 209 | 352 | 4 | 1.6× faster |
| ulule/deepcopier | 6,296 | 5,760 | 64 | **19.3× slower** |
| encoding/json | 9,549 | 1,432 | 35 | **29.2× slower** |

### Deep Struct (Organisation: 10 employees, circular references)

| Library | ns/op | Handles cycles? |
|---------|------:|:---------------:|
| Manual (baseline) | 7,028 | ✅ (explicit) |
| **FastCopier (with gen)** | 2,206 | **✅** |
| FastCopier.Clone | 2,381 | ✅ |
| huandu/go-clone | ❌ stack overflow | ❌ |
| tiendc/go-deepcopy | ❌ stack overflow | ❌ |
| jinzhu/copier | 3,997 | ⚠️ shallow ptrs |
| go-viper/mapstructure | ❌ stack overflow | ❌ |
| ulule/deepcopier | 10,761 | ⚠️ shallow ptrs |
| encoding/json | ❌ infinite loop | ❌ |

> **FastCopier with generated code matches manual copy on Complex.**
> FastCopier is the **only** library that both completes the deep copy correctly **and** handles pointer cycles.  
> `⚠️ shallow ptrs` = pointer fields are copied as-is (same address), not recursively cloned.

**Allocation notes:**
- **Structs and slices:** zero allocations on repeated calls (`sync.Pool` + slice capacity reuse)
- **Maps:** unavoidable allocation per call (new map required every time)
- **First call:** allocates the copy plan; all subsequent calls reuse it from the sharded cache

See [BENCHMARKS.md](BENCHMARKS.md) for the full comparison including the code-generation tier and safety matrix.

<!-- BENCHMARK_RESULTS_END -->

## Features

- ✅ **Zero allocations** for structs and slices on repeated calls (`sync.Pool` + slice capacity reuse)
- ✅ **Native slice/map support** — `Copy(&dstSlice, &srcSlice)` works directly, no loops needed
- ✅ **`Map[Src, Dst]`** — generic helper for `[]Src → []Dst` conversion without pre-declaring destination
- ✅ **`Merge`** — PATCH semantics: only non-zero source fields overwrite destination
- ✅ **`Inspect`** — human-readable plan showing exactly what will be copied and how
- ✅ **`MustRegister`** — fail-fast startup validator to catch field mismatches before production
- ✅ **Structured errors** — `*CopyError` with field-level context; `errors.Is` / `errors.As` compatible
- ✅ **Flat struct fast path** — same-type structs with only scalar fields copied via a single `reflect.Value.Set`
- ✅ **Flat-struct-slice bulk copy** — `[]FlatStruct`, `[]string`, `[]int`, etc. copied with a single `reflect.Copy` call
- ✅ **`RegisterCopier`** — register a hand-written or generated zero-reflection copy function for any type pair
- ✅ **`fastcopier-gen`** — optional CLI to generate `RegisterCopier` calls automatically from your Go types
- ✅ **No code generation required** — pure reflection engine works out of the box with any struct
- ✅ **Intelligent caching** — copy plans built once per type pair, reused forever
- ✅ **Circular reference detection** — prevents infinite loops on pointer cycles
- ✅ **Struct tag support** — `fastcopier:"field_name"` or `fastcopier:"-"` to skip
- ✅ **Embedded struct support** with field promotion
- ✅ **Type conversion** between compatible types
- ✅ **Deep copy** for nested structs, slices, and maps
- ✅ **Concurrent-safe** global cache

## Quick Start

```go
import "github.com/expego/fastcopier"

type Source struct {
    Name  string
    Age   int
    Email string
}

type Destination struct {
    Name  string
    Age   int
    Email string
}

func main() {
    src := Source{Name: "John", Age: 30, Email: "john@example.com"}
    var dst Destination

    if err := fastcopier.Copy(&dst, &src); err != nil {
        log.Fatal(err)
    }
}
```

### Slice and Map Copying

`Copy` handles slices and maps natively — no loops required:

```go
// []Src → []Dst (different types)
var dstUsers []UserDTO
fastcopier.Copy(&dstUsers, &srcUsers)

// Generic helper — no pre-declaration needed
dtos, err := fastcopier.Map[UserEntity, UserDTO](entities)

// map copy
var dstMap map[string]string
fastcopier.Copy(&dstMap, &srcMap)
```

### Merge (PATCH semantics)

Only non-zero source fields overwrite the destination — ideal for REST PATCH endpoints:

```go
existing := User{Name: "Alice", Age: 30, Email: "alice@example.com"}
patch := User{Email: "new@example.com"} // only Email is non-zero

fastcopier.Merge(&existing, &patch)
// existing.Name == "Alice", existing.Age == 30, existing.Email == "new@example.com"
```

### Inspect — Audit Your Mappings

See exactly what will be copied before it runs. Useful for debugging silent field mismatches and for AI coding agents:

```go
plan, err := fastcopier.Inspect(&UserDTO{}, &UserEntity{})
if err != nil {
    log.Fatal(err)
}
fmt.Print(plan)
// Copy fastcopier_test.UserEntity → fastcopier_test.UserDTO
//   ID                   → ID                     [int]  assign
//   Name                 → Name                   [string]  assign
//   skipped (no dst match): [Email]
```

### MustRegister — Fail Fast at Startup

Pre-build and validate copy plans at program startup. Panics immediately if a mapping is invalid:

```go
func init() {
    fastcopier.MustRegister(&UserDTO{}, &UserEntity{})
    fastcopier.MustRegister(&OrderDTO{}, &OrderEntity{})
}
```

### Struct Tags

```go
type Source struct {
    UserName string `fastcopier:"Name"`  // map to different field name
    UserAge  int    `fastcopier:"Age"`
    Internal string `fastcopier:"-"`     // skip this field
}

type Destination struct {
    Name string
    Age  int
}
```

> **Migration from `jinzhu/copier`:** The default tag key is `fastcopier` (not `copier`) to avoid
> silent conflicts when both libraries are present. To keep using `copier` tags, call
> `fastcopier.SetDefaultTagName("copier")` once at startup.

### Embedded Structs

Embedded struct fields are automatically promoted:

```go
type Base struct {
    ID   int
    Name string
}

type Source struct {
    Base
    Email string
}

type Destination struct {
    ID    int
    Name  string
    Email string
}
```

### Options

```go
// Disable copying between pointers and values
fastcopier.Copy(&dst, &src, fastcopier.WithCopyBetweenPtrAndValue(false))

// Silently skip non-copyable types instead of returning an error
fastcopier.Copy(&dst, &src, fastcopier.WithIgnoreNonCopyableTypes(true))

// Only copy specific fields
fastcopier.Copy(&dst, &src, fastcopier.WithFields("Name", "Email"))

// Skip zero-value source fields (PATCH semantics)
fastcopier.Copy(&dst, &src, fastcopier.WithSkipZeroFields(true))
```

### Structured Errors

Errors carry field-level context for precise diagnostics:

```go
err := fastcopier.Copy(&dst, &src)

var ce *fastcopier.CopyError
if errors.As(err, &ce) {
    fmt.Printf("failed: %s → %s (field %q): %v\n",
        ce.SrcType, ce.DstType, ce.SrcField, ce.Err)
}

// Sentinel errors still work with errors.Is:
if errors.Is(err, fastcopier.ErrTypeNonCopyable) { ... }
```

## How It Beats the Competition

### vs tiendc/go-deepcopy

go-deepcopy is a well-engineered library. FastCopier beats it through:

1. **Flat struct fast path** — structs composed entirely of scalar fields are detected at plan-build time. Their copy reduces to a single `reflect.Value.Set` call instead of N per-field operations.
2. **Flat-struct-slice bulk copy** — `[]FlatStruct`, `[]string`, `[]int`, etc. are copied with a single `reflect.Copy` call rather than element-by-element iteration through the reflect interface.
3. **`sync.Pool` for Context objects** — eliminates the per-call heap allocation for the copy context.
4. **Lazy circular-reference tracking** — the `visited` map is only allocated when a pointer cycle is actually encountered, not on every call.
5. **In-place slice resize** — when the destination slice has sufficient capacity, the backing array is reused by updating the slice header in-place, avoiding `reflect.MakeSlice`.
6. **Upfront pointer dereference** — the source pointer is dereferenced once in `Copy()`, eliminating a `ptr2ValueCopier` wrapper in the hot path.
7. **`RegisterCopier` / code generation** — for hot paths, `fastcopier-gen` generates plain field-assignment functions that are registered at `init()`. `fastcopier.Copy` routes to them automatically, bypassing reflection entirely and reaching parity with hand-written code (166 ns vs 163 ns on Complex).

### vs jinzhu/copier and ulule/deepcopier

Both use unoptimized reflection with no caching. FastCopier caches the entire copy plan (which fields to copy, how to copy them) per type pair on first use, so subsequent calls pay only the cost of executing the plan — no type inspection, no map lookups, no string comparisons.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for a full walkthrough of the algorithm — entry-point flow, plan resolution tree, caching design, circular-reference detection, and flat-type fast paths.

See [docs/RESEARCH.md](docs/RESEARCH.md) for design decisions and [docs/BENCHMARKS.md](docs/BENCHMARKS.md) for detailed results.

## License

MIT
