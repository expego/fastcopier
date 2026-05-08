# Benchmark Results

## Test Environment

- **CPU**: Apple M4
- **OS**: darwin/arm64
- **Go Version**: 1.26.2
- **Benchmark Time**: 3 seconds per test (`-benchtime=3s`)
- **Method**: All libraries copy the same fully-populated source struct into a zero-value
  destination — same direction, same data.

## Libraries Under Test

| Library | Version | Stars | Notes |
|---------|---------|-------|-------|
| **FastCopier** | (this repo) | — | Reflection + plan cache + optional codegen |
| huandu/go-clone | v1.7.3 | ~900 | Unsafe-accelerated deep clone |
| tiendc/go-deepcopy | v1.2.1 | ~600 | Reflection-based deep copy |
| go-viper/mapstructure | v2.5.0 | ~8k | Struct/map decoder (Viper's mapper) |
| jinzhu/copier | v0.4.0 | ~5.5k | Reflection-based struct mapper |
| ulule/deepcopier | v0.0.0 | ~1k | Tag-driven struct mapper |
| encoding/json | stdlib | — | JSON marshal+unmarshal round-trip (common naive pattern) |

Two FastCopier modes are shown:

| Mode | Description | Build |
|------|-------------|-------|
| **FastCopier** | `RegisterCopier` called at `init()` from generated code | *(default)* |
| **FastCopier (reflect)** | Generated code excluded | `-tags fastcopier_no_gen` |

---

## Results

### Simple Struct (5 primitive fields)

```go
type BenchSimple struct {
    Name   string
    Age    int
    Email  string
    Score  float64
    Active bool
}
```

| Library | ns/op | B/op | allocs/op | vs FastCopier |
|---------|------:|-----:|----------:|:-------------:|
| Manual (baseline) | 0.26 | 0 | 0 | 196× faster |
| **FastCopier (with gen)** | **51.0** | **0** | **0** | **—** |
| FastCopier (pure reflect) | 91.5 | 0 | 0 | 1.8× slower |
| FastCopier.Clone | 76.8 | 128 | 2 | 1.5× slower |
| huandu/go-clone | 71.7 | 128 | 2 | 1.4× slower |
| go-viper/mapstructure | 73.5 | 176 | 3 | 1.4× slower |
| tiendc/go-deepcopy | 99.8 | 32 | 1 | 2.0× slower |
| encoding/json | 748.9 | 336 | 7 | **15× slower** |
| jinzhu/copier | 1,254 | 496 | 18 | **25× slower** |
| ulule/deepcopier | 2,543 | 5,712 | 62 | **50× slower** |

### Nested Struct (struct + slices)

```go
type BenchNested struct {
    ID      int
    Profile BenchSimple
    Tags    []string   // 3 elements
    Scores  []int      // 5 elements
}
```

| Library | ns/op | B/op | allocs/op | vs FastCopier |
|---------|------:|-----:|----------:|:-------------:|
| Manual (baseline) | 24.3 | 96 | 2 | 2.4× faster |
| **FastCopier (with gen)** | **57.3** | **0** | **0** | **—** |
| FastCopier (pure reflect) | 156.4 | 0 | 0 | 2.7× slower |
| go-viper/mapstructure | 94.8 | 288 | 4 | 1.7× slower |
| FastCopier.Clone | 115.6 | 320 | 4 | 2.0× slower |
| huandu/go-clone | 194.3 | 480 | 7 | 3.4× slower |
| tiendc/go-deepcopy | 266.0 | 176 | 5 | 4.6× slower |
| jinzhu/copier | 1,057 | 600 | 16 | **18× slower** |
| encoding/json | 1,732 | 608 | 13 | **30× slower** |
| ulule/deepcopier | 1,820 | 3,744 | 41 | **32× slower** |

### Complex Struct (nested + slice of structs + map)

```go
type BenchComplex struct {
    ID       int
    Name     string
    Nested   BenchNested
    Items    []BenchSimple     // 3 elements
    Metadata map[string]string // 3 entries
}
```

| Library | ns/op | B/op | allocs/op | vs FastCopier |
|---------|------:|-----:|----------:|:-------------:|
| Manual (baseline) | 158.5 | 568 | 5 | on-par |
| **FastCopier (with gen)** | **169.3** | **336** | **2** | **—** |
| go-viper/mapstructure¹ | 107.7 | 352 | 4 | 1.6× faster |
| FastCopier (pure reflect) | 393.6 | 96 | 6 | 2.3× slower |
| FastCopier.Clone | 275.9 | 920 | 7 | 1.6× slower |
| tiendc/go-deepcopy | 683.4 | 432 | 13 | 4.0× slower |
| huandu/go-clone | 815.7 | 1,568 | 21 | 4.8× slower |
| jinzhu/copier | 1,288 | 720 | 18 | **7.6× slower** |
| ulule/deepcopier | 2,583 | 5,712 | 62 | **15× slower** |
| encoding/json | 4,626 | 1,432 | 35 | **27× slower** |

> ¹ `go-viper/mapstructure` appears faster here because it **shares** the `map[string]string`
> reference instead of deep-copying it. FastCopier allocates and populates a new map, which
> is the correct behaviour for a deep copy. When correctness matters, mapstructure's speed
> advantage disappears.

> **FastCopier with generated code (169 ns) is statistically at parity with manual copy (158 ns).**  
> The 2 allocs come from `map[string]string` creation — unavoidable without map reuse.

### Deep Struct (Organisation: 10 employees, nested pointers, maps, circular refs)

```go
type Organisation struct {
    ID          int
    Name        string
    Founded     int
    Departments []Department      // 3 entries; each Department.Manager *Employee
    Employees   []Employee        // 10 entries; each Employee.ReportsTo *Employee (cycle!)
    Metadata    map[string]string
    HeadOffice  Address
}
```

| Library | ns/op | B/op | allocs/op | Handles cycles? |
|---------|------:|-----:|----------:|:-----------:|
| Manual (baseline) | 827.4 | 4,936 | 20 | ✅ (explicit) |
| **FastCopier** | **991.3** | **1,104** | **18** | **✅** |
| FastCopier.Clone | 1,053 | 1,425 | 20 | ✅ |
| jinzhu/copier | 1,887 | 704 | 22 | ⚠️ shallow ptrs |
| ulule/deepcopier | 4,428 | 10,496 | 114 | ⚠️ shallow ptrs |
| tiendc/go-deepcopy | ❌ stack overflow | — | — | ❌ |
| go-viper/mapstructure | ❌ stack overflow | — | — | ❌ |
| huandu/go-clone | ❌ stack overflow | — | — | ❌ |
| encoding/json | ❌ infinite loop | — | — | ❌ |

> **Cycle column notes:**
> - ✅ **True deep copy** with cycle detection — pointer targets are recursively copied; revisited pointers reuse the already-copied target.
> - ⚠️ **Shallow pointers** — pointer fields are copied as raw pointer values (same address). The pointed-to data is shared, not cloned. These libraries "succeed" because they never recurse into the pointed-to struct.
> - ❌ **Crash / hang** — no cycle guard; stack overflows or infinite loops on the `Employee.ReportsTo *Employee` cycle.
>
> FastCopier is the **only** library that both completes the deep copy correctly *and* handles pointer cycles.

---

## Code-Generation Tier (FastCopier-specific)

Running the generated copy functions directly (bypassing the `fastcopier.Copy` dispatch):

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| GeneratedDirect_Simple | 0.25 | 0 | 0 |
| Generated_Via_Copy_Simple | 49.4 | 0 | 0 |
| GeneratedDirect_Nested | 5.03 | 0 | 0 |
| Generated_Via_Copy_Nested | 54.9 | 0 | 0 |
| GeneratedDirect_Complex | 116.4 | 336 | 2 |
| Generated_Via_Copy_Complex | 165.7 | 336 | 2 |

The `~50 ns` overhead in `Via_Copy` vs `Direct` is the cost of the `sync.Map` registry lookup in `resolvePlan`. Callers who need sub-nanosecond copies can call the generated `CopyXtoY` function directly.

---

## Key Optimizations

### 1. Flat struct fast path

Structs whose every field is a scalar (no slices, maps, pointers, interfaces, or channels)
are detected once at plan-build time and classified as *flat*. Their copy reduces to a
**single `reflect.Value.Set` call** instead of N per-field calls:

```go
if dstType == srcType && isFlatType(srcType) {
    return defaultAssignPlan, nil  // one Set() replaces N field plans
}
```

The same check applies inside `buildFieldPlan`: a flat struct field (e.g. `Profile BenchSimple`
inside `BenchNested`) also uses a direct `Set` with no inner-plan dispatch.

### 2. Flat-struct-slice bulk copy

Slices whose element type is flat (including `[]FlatStruct`, `[]string`, `[]int`, etc.) are
bulk-copied with a single `reflect.Copy` call — equivalent to the built-in `copy` — instead
of element-by-element iteration:

```go
if dstElem == srcElem && isFlatType(srcElem) {
    p.bulkCopy = true   // uses reflect.Copy for the entire slice
}
```

### 3. `sync.Pool` for Context (eliminates alloc/call)

```go
var ctxPool = sync.Pool{New: func() any { return &Context{...} }}
```

### 4. Lazy circular-reference map (eliminates alloc/call for non-pointer structs)

```go
if c.ctx.visited == nil {
    c.ctx.visited = make(map[uintptr]bool, 4)
}
```

### 5. In-place slice resize (eliminates alloc on repeated copies)

```go
if dst.Cap() >= srcLen {
    dst.SetLen(srcLen)
    reflect.Copy(dst, src)
    return nil
}
```

### 6. `RegisterCopier` / code generation (optional, maximum performance)

```go
fastcopier.RegisterCopier(func(dst, src *BenchSimple) error { *dst = *src; return nil })
```

`fastcopier-gen` generates these registrations automatically from Go source types.
`fastcopier.Copy` checks the registry first (one `sync.Map` lookup) and dispatches to the
registered function, bypassing the reflection engine entirely.

---

## Safety Comparison

| Feature | FastCopier | go-deepcopy | go-clone | mapstructure | jinzhu/copier | ulule |
|---------|:----------:|:-----------:|:--------:|:------------:|:-------------:|:-----:|
| Circular reference detection | ✅ | ❌ crash | ❌ crash | ❌ crash | ⚠️ shallow | ⚠️ shallow |
| True deep copy of pointers | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Zero allocations (no maps) | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Flat struct fast path | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Embedded struct promotion | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Struct tag support | ✅ | ✅ | — | ✅ | ✅ | ✅ |
| No code generation needed | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Optional code generation | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
