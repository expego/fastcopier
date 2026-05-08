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

Benchmarks run on Apple M4, go1.26.3, `-benchtime=1s`. All FastCopier results produce **genuine independent copies** — mutating the source after copying does not affect the destination.

### Simple Struct (5 primitive fields)

| Library | ns/op | B/op | allocs/op | vs FastCopier |
|---------|------:|-----:|----------:|:-------------:|
| Manual (baseline) | 0.25 | 0 | 0 | — |
| **FastCopier (pure reflect)** | **53** | **0** | **0** | **—** |
| FastCopier.Clone | 81 | 128 | 2 | 1.5× slower |
| huandu/go-clone | 73 | 128 | 2 | 1.4× slower |
| tiendc/go-deepcopy | 109 | 32 | 1 | 2.1× slower |
| jinzhu/copier | 1,265 | 496 | 18 | **23.9× slower** |
| go-viper/mapstructure | 86 | 176 | 3 | 1.6× slower |
| ulule/deepcopier | 2,673 | 5,712 | 62 | **50.4× slower** |
| encoding/json | 769 | 336 | 7 | **14.5× slower** |

### Nested Struct (struct + slices)

| Library | ns/op | B/op | allocs/op | vs FastCopier |
|---------|------:|-----:|----------:|:-------------:|
| Manual (baseline) | 26 | 96 | 2 | 2.1× faster |
| **FastCopier (pure reflect)** | **55** | **0** | **0** | **—** |
| FastCopier.Clone | 119 | 320 | 4 | 2.2× slower |
| huandu/go-clone | 194 | 480 | 7 | 3.5× slower |
| tiendc/go-deepcopy | 266 | 176 | 5 | 4.8× slower |
| jinzhu/copier | 1,060 | 600 | 16 | **19.3× slower** |
| go-viper/mapstructure | 102 | 288 | 4 | 1.9× slower |
| ulule/deepcopier | 1,926 | 3,744 | 41 | **35.0× slower** |
| encoding/json | 1,773 | 608 | 13 | **32.2× slower** |

### Complex Struct (nested + slice of structs + map)

> **What counts as a real deep copy?** After copying, mutating `src.Items[0]`, `src.Nested.Tags[0]`, or `src.Metadata["key"]` must **not** affect `dst`. Libraries that copy slice/map headers without allocating new backing storage are cheaper but silently share memory with the source.

| Library | ns/op | B/op | allocs/op | Real deep copy? |
|---------|------:|-----:|----------:|:---------------:|
| Manual (baseline) | 205 | 568 | 5 | ✅ |
| **FastCopier (pure reflect)** | **300** | **336** | **2** | **✅** |
| FastCopier.Clone | 412 | 920 | 7 | ✅ |
| huandu/go-clone | 856 | 1,568 | 21 | ✅ |
| tiendc/go-deepcopy | 749 | 432 | 13 | ✅ |
| jinzhu/copier | 1,388 | 720 | 18 | ✅ |
| go-viper/mapstructure | 134 | 352 | 4 | ❌ shallow¹ |
| ulule/deepcopier | 2,884 | 5,712 | 62 | ✅ |
| encoding/json | 4,990 | 1,432 | 35 | ✅ |

> ¹ `mapstructure.Decode` copies slice and map *headers* only — `src` and `dst` share the same backing array and map. Mutating `src` after the call corrupts `dst`. Verified: `src.Tags[0] = "CHANGED"` causes `dst.Tags[0] == "CHANGED"`. FastCopier allocates a new map and copies each slice element independently, which accounts for the difference in cost.

**Among libraries that perform a correct deep copy, FastCopier is the fastest** — 1.5× faster than `huandu/go-clone` (856 ns), 2.5× faster than `tiendc/go-deepcopy` (749 ns), and 4.6× faster than `jinzhu/copier` (1,388 ns).

### Deep Struct (Organisation: 10 employees, circular references)

| Library | ns/op | Handles cycles? |
|---------|------:|:---------------:|
| Manual (baseline) | 3,367 | ✅ (explicit) |
| **FastCopier (pure reflect)** | **973** | **✅** |
| FastCopier.Clone | 1,056 | ✅ |
| huandu/go-clone | ❌ stack overflow | ❌ |
| tiendc/go-deepcopy | ❌ stack overflow | ❌ |
| jinzhu/copier | 1,804 | ⚠️ shallow ptrs |
| go-viper/mapstructure | ❌ stack overflow | ❌ |
| ulule/deepcopier | 4,594 | ⚠️ shallow ptrs |
| encoding/json | ❌ infinite loop | ❌ |

> FastCopier is the **only** library that both completes the deep copy correctly **and** handles pointer cycles — beating even the hand-written manual baseline (3,367 ns) by 3.5×.  
> `⚠️ shallow ptrs` = pointer fields are copied as-is (same address), not recursively cloned.

**Allocation notes:**
- **Structs and slices:** zero allocations on repeated calls (`sync.Pool` + slice capacity reuse)
- **Maps:** one allocation per call — a new map must be created to avoid sharing with the source
- **First call:** allocates the copy plan; all subsequent calls reuse it from the sharded cache

See [docs/BENCHMARKS.md](docs/BENCHMARKS.md) for full comparison including code-generation tier and safety matrix.

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
