# FastCopier — Architecture & Algorithm

## Overview

FastCopier is a two-phase deep-copy engine:

1. **Plan phase** — inspect source/destination types once, build a tree of `Plan` objects, cache it.
2. **Execute phase** — on every subsequent call, just run the cached plan tree against live values.

---

## Entry Point Flow

```
fastcopier.Copy(&dst, src)
        │
        ▼
validateAndDeref()
  ├── dst must be non-nil pointer        → ErrValueInvalid / ErrTypeInvalid
  ├── src must be non-nil                → ErrValueInvalid
  └── dereference src pointer once       → srcVal (never a pointer)
        │
        ▼
ctxPool.Get()  ←──────────────────────────────────────────────┐
  └── ctx.prepare()                                            │
        ├── ctx.flags  (bitmask: CopyBetweenPtrAndValue,       │
        │               IgnoreNonCopyableTypes, SkipZeroFields) │
        └── ctx.cache  (globalCache or per-call simpleCache)    │
              │                                                 │
              ▼                                                 │
        resolvePlan(ctx, dstType, srcType)  ◄── cache hit?     │
              │                                                 │
              ▼                                                 │
        plan.Copy(dstVal, srcVal, ctx)                         │
              │                                                 │
              ▼                                                 │
        ctx.reset() → ctxPool.Put(ctx) ────────────────────────┘
```

---

## Plan Resolution Tree

`resolvePlan` is called recursively during **plan-build time** only (not at copy time).
Each node type is resolved once and cached under `(dstType, srcType, flags)`.

```
resolvePlan(dstType, srcType)
│
├── Registered copier?  ──────────────────► registered function (zero reflection)
│
├── Cache hit?  ───────────────────────────► cached Plan  (O(1) sharded RW lock)
│
├── src is scalar / func
│   ├── dstType == srcType  ───────────────► assignPlan
│   └── ConvertibleTo       ───────────────► convertPlan
│
├── src is Chan  ──────────────────────────► chanPlan  (shared reference)
│
├── dst is Interface  ─────────────────────► ifaceDstPlan
│   └── at copy time: clone concrete value into interface
│
├── src is Interface  ─────────────────────► ifaceSrcPlan
│   └── at copy time: unwrap, dispatch to concrete plan
│
├── src/dst is Pointer
│   ├── *T → *T   ────────────────────────► ptrPlan  (alloc dst if nil, cycle check)
│   ├── *T → V    ─── CopyBetweenPtr? ───► derefPlan
│   └── V  → *T   ─── CopyBetweenPtr? ───► addrPlan (alloc *T, copy V in)
│
├── src is Slice / Array
│   ├── same flat elem type  ─────────────► slicePlan{bulkCopy: true}
│   │   └── reflect.Copy (single memcpy)
│   └── complex elem type   ─────────────► slicePlan{bulkCopy: false}
│       └── per-element resolvePlan(elem)
│
├── src is Struct
│   ├── dst is Struct
│   │   ├── same type + flat + no options ► assignPlan  (single Set = deep copy)
│   │   └── otherwise  ──────────────────► structPlan
│   │       └── per-field fieldPlan list
│   └── dst is Map   ────────────────────► structToMapPlan
│       └── per-field field2MapPlan list
│
└── src is Map
    ├── dst is Map    ───────────────────► mapPlan
    └── dst is Struct ───────────────────► mapToStructPlan
        └── per-key val2FieldPlan list
```

---

## Struct Copy: Field Matching

When `structPlan.init` runs, it builds a field list once per `(dstType, srcType)` pair:

```
parseStructFields(srcType)          parseStructFields(dstType)
       │                                      │
       ▼                                      ▼
 srcDirect  map[key]*fieldMeta    dstDirect  map[key]*fieldMeta
 srcInherited (embedded)          dstInherited (embedded)
       │                                      │
       └──────────────── match by key ────────┘
                               │
               ┌───────────────┼───────────────────┐
               │               │                   │
          same flat type   scalar convert    recurse resolvePlan
               │               │                   │
          fieldPlan         fieldPlan           fieldPlan
         (elem=nil,        (elem=convert)    (elem=inner plan)
          single Set)
```

**Key resolution order:**
1. Struct tag `fastcopier:"name"` overrides field name
2. `fastcopier:"-"` skips the field entirely
3. Embedded / anonymous struct fields are promoted (with ambiguity detection)

---

## Plan Types Reference

| Plan | Source → Dest | Allocates? | Notes |
|------|--------------|:----------:|-------|
| `assignPlan` | T → T | No | Single `Set` call |
| `convertPlan` | T → U | No | `src.Convert(dstType)` |
| `chanPlan` | chan → chan | No | Shared reference |
| `skipPlan` | any → any | No | No-op |
| `ptrPlan` | \*T → \*T | On nil dst | Circular-ref guard |
| `derefPlan` | \*T → V | No | Dereferences src |
| `addrPlan` | V → \*T | Yes (once) | Allocates \*T |
| `deferredPlan` | T → T | No | Breaks build-time cycles |
| `ifaceSrcPlan` | iface → T | No | Runtime dispatch |
| `ifaceDstPlan` | T → iface | Clone alloc | Clones concrete value |
| `slicePlan` (bulk) | []T → []T | Only if cap insufficient | `reflect.Copy` |
| `slicePlan` (elem) | []S → []D | Only if cap insufficient | Per-element |
| `structPlan` | S → D | No | Per-field list |
| `mapPlan` | map → map | Yes | New map every time |
| `structToMapPlan` | S → map | Yes | New map every time |
| `mapToStructPlan` | map → S | No | Per-key dispatch |

---

## Caching Architecture

```
┌─────────────────────────────────────────────────────┐
│              globalCache (shardedCache)              │
│                                                      │
│  64 shards, each:  sync.RWMutex + map[planKey]Plan  │
│                                                      │
│  planKey = (dstType, srcType, flags uint8)           │
│                                                      │
│  hash = typeHash(dst) * A                            │
│       ^ typeHash(src) * B                            │
│       ^ uint64(flags) * C        → shard index       │
│                                                      │
│  typeHash: Fibonacci hash of reflect.Type data ptr   │
│  → allocation-free, collision-free for distinct types │
└─────────────────────────────────────────────────────┘

  WithTagName / WithFields?          ctx.UseGlobalCache = false
       │                                      │
       └──────────────────────────────────────┘
                          │
                          ▼
              simpleCache (plain map, per-call)
```

---

## Context Lifecycle & sync.Pool

```
ctxPool (sync.Pool)        mergeCtxPool (sync.Pool)
      │                            │
      │  Copy() / Clone()          │  Merge() fast path
      ▼                            ▼
  ctx = pool.Get()            ctx = pool.Get()
  ctx.prepare()               ctx.SkipZeroFields = true
      │                       ctx.prepare()
      │                            │
      ├── plan build / execute ────┤
      │                            │
  ctx.reset()  ◄──────────────────┘
  pool.Put(ctx)
      │
      ▼
  zero allocations on repeated calls (structs & slices)
```

`ctx.reset()` is the single source of truth for defaults — it clears **all** fields so no stale state leaks between callers sharing the pool.

---

## Circular Reference Detection

```
ptrPlan.Copy(dst, src)
      │
      ▼
checkCircularRef(src, ctx)
      │
      ├── src.IsNil? → skip
      │
      ├── ptr = src.Pointer()
      │
      ├── ctx.visited[ptr]?  ← nil-map read is safe (returns false)
      │       │
      │   ┌───┴──────────────────────────────────┐
      │   │ cycle detected                       │
      │   ├── CircularRefSkip → dst = zero, nil  │
      │   └── CircularRefError → ErrCircularReference
      │
      ├── ctx.visited == nil? → lazy alloc map[uintptr]bool
      │
      ├── ctx.visited[ptr] = true
      │
      └── defer delete(ctx.visited, ptr)  ← unwind on return
```

`visited` is **lazily allocated** — the map is only created when the first pointer is encountered, not on every `Copy` call.

---

## Flat-Type Fast Paths

FastCopier detects structs/slices that contain no heap-allocated fields at plan-build time:

```
isFlatType(T)?   (cached in flatCache sync.Map)
      │
      ├── scalar / func kind  →  true
      ├── array               →  isFlatType(elem)
      ├── struct              →  all fields flat?
      └── slice/map/ptr/iface →  false

Fast paths enabled when isFlatType:
  ┌──────────────────────────────────────────────────┐
  │  struct copy: single reflect.Value.Set()         │
  │  (same type + flat + no SkipZeroFields/fieldMask)│
  └──────────────────────────────────────────────────┘
  ┌──────────────────────────────────────────────────┐
  │  slice copy: single reflect.Copy()               │
  │  (same elem type + flat → one memcpy)            │
  └──────────────────────────────────────────────────┘
```

---

## Key Optimisations Summary

| Optimisation | Where | Effect |
|---|---|---|
| Plan cache (64 shards) | `engine.go` | Build plan once, reuse forever |
| `sync.Pool` for `Context` | `fastcopier.go` | Zero per-call heap allocation |
| Dedicated `mergeCtxPool` | `fastcopier.go` | `Merge` avoids `[]Option` alloc |
| Flat-struct `Set` fast path | `engine.go` | N field ops → 1 `Set` |
| Flat-slice `reflect.Copy` | `plan_slice.go` | N elem ops → 1 memcpy |
| In-place slice resize | `plan_slice.go` | Reuse backing array if cap sufficient |
| Lazy `visited` map | `plans.go` | No alloc unless cycle encountered |
| Upfront src deref | `fastcopier.go` | Eliminates `ptr2ValueCopier` wrapper |
| Fibonacci hash sharding | `engine.go` | Low-contention concurrent cache reads |
| `RegisterCopier` / codegen | `registry.go` | Bypasses reflection entirely |
