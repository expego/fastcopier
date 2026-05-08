package fastcopier

import (
	"reflect"
	"sync"
	"unsafe"
)

// planKey uniquely identifies a Plan for a (dstType, srcType, flags) triple.
type planKey struct {
	dstType reflect.Type
	srcType reflect.Type
	flags   uint8
}

// planCache is the interface satisfied by both shardedCache (global, concurrent-safe)
// and simpleCache (per-call, lock-free).
type planCache interface {
	get(key planKey) (Plan, bool)
	set(key planKey, p Plan)
	del(key planKey)
}

// ── Sharded plan cache (global) ───────────────────────────────────────────────

const numShards = 64

type planShard struct {
	mu sync.RWMutex
	m  map[planKey]Plan
}

type shardedCache [numShards]planShard

func newShardedCache() *shardedCache {
	sc := &shardedCache{}
	for i := range sc {
		sc[i].m = make(map[planKey]Plan, 4)
	}
	return sc
}

// typeHash returns a stable hash for a reflect.Type using its interface data pointer.
// reflect.Type values for the same type are always the same pointer, so this is
// both allocation-free and collision-free for distinct types.
func typeHash(t reflect.Type) uint64 {
	// reflect.Type is an interface; extract the data pointer for a stable identity hash.
	type iface struct{ _, data uintptr }
	p := (*iface)(unsafe.Pointer(&t))
	// Mix with a Fibonacci-hashing constant for good bit distribution.
	h := uint64(p.data) * 11400714819323198485
	return h ^ (h >> 30)
}

func (sc *shardedCache) shard(key planKey) *planShard {
	h := typeHash(key.dstType)*2654435761 ^ typeHash(key.srcType)*2246822519 ^ uint64(key.flags)*374761393
	return &sc[h&(numShards-1)]
}

func (sc *shardedCache) get(key planKey) (Plan, bool) {
	s := sc.shard(key)
	s.mu.RLock()
	p, ok := s.m[key]
	s.mu.RUnlock()
	return p, ok
}

func (sc *shardedCache) set(key planKey, p Plan) {
	s := sc.shard(key)
	s.mu.Lock()
	s.m[key] = p
	s.mu.Unlock()
}

func (sc *shardedCache) del(key planKey) {
	s := sc.shard(key)
	s.mu.Lock()
	delete(s.m, key)
	s.mu.Unlock()
}

func (sc *shardedCache) clear() {
	for i := range sc {
		sc[i].mu.Lock()
		sc[i].m = make(map[planKey]Plan, 4)
		sc[i].mu.Unlock()
	}
}

var globalCache = newShardedCache()

// ── Simple per-call cache (lock-free) ─────────────────────────────────────────

// simpleCache is a lock-free plan cache used for non-global (per-call) plan
// resolution. It is never shared across goroutines so no locking is required.
// Using a plain map is significantly cheaper than allocating the 64-shard
// shardedCache for a temporary, single-use cache.
type simpleCache map[planKey]Plan

func (c simpleCache) get(key planKey) (Plan, bool) {
	p, ok := c[key]
	return p, ok
}

func (c simpleCache) set(key planKey, p Plan) {
	c[key] = p
}

func (c simpleCache) del(key planKey) {
	delete(c, key)
}

// flatCache memoises isFlatType results to avoid repeated field traversal.
var flatCache sync.Map // map[reflect.Type]bool

// isFlatType reports whether t contains no heap-allocated fields,
// meaning reflect.Value.Set produces a correct deep copy.
// Flat types: scalars (bool, int*, uint*, float*, complex*, string, uintptr, func),
// arrays of flat elements, and structs whose every field is flat.
// Slices, maps, pointers, interfaces, and channels are never flat.
func isFlatType(t reflect.Type) bool {
	if v, ok := flatCache.Load(t); ok {
		return v.(bool)
	}
	result := computeFlatType(t)
	flatCache.Store(t, result)
	return result
}

func computeFlatType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if !isFlatType(t.Field(i).Type) {
				return false
			}
		}
		return true
	case reflect.Array:
		return isFlatType(t.Elem())
	default:
		return scalarKinds&(1<<t.Kind()) > 0
	}
}

var (
	// scalarKinds is a bitmask for O(1) primitive-kind checks.
	scalarKinds = func() uint32 {
		n := uint32(0)
		for _, k := range []reflect.Kind{
			reflect.Bool,
			reflect.String,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64,
			reflect.Complex64, reflect.Complex128,
			reflect.Uintptr,
			// reflect.Func: functions are copied by reference (same as channels).
			// Including Func here lets isFlatType treat func fields as flat (no heap traversal needed).
			reflect.Func,
		} {
			n |= 1 << k
		}
		return n
	}()
)

const (
	// flagBit* constants are bit positions (shift amounts) used to build ctx.flags.
	// Bits are intentionally non-contiguous to leave room for future flags without
	// invalidating cached plan keys. The current uint8 flags field supports bit
	// positions 0–7; if more than two additional boolean flags are needed, widen
	// flags to uint16 and update planKey accordingly.
	//
	// Current layout (bit positions):
	//   0 — unused (reserved)
	//   1 — CopyBetweenPtrAndValue
	//   2 — unused (reserved)
	//   3 — IgnoreNonCopyableTypes
	//   4 — unused (reserved)
	//   5 — SkipZeroFields
	//   6,7 — unused (reserved)
	flagBitCopyBetweenPtrAndValue = 1
	flagBitIgnoreNonCopyableTypes = 3
	// flagBitSkipZeroFields separates cached plans for Merge/SkipZeroFields from
	// regular Copy plans.  The flat-struct fast path must be skipped when this flag
	// is set so that per-field zero checks are evaluated at runtime.
	flagBitSkipZeroFields = 5
)

func defaultCtx() *Context {
	// Pull from the pool so callers that apply options also benefit from pooling.
	// The caller is responsible for calling ctx.reset() and returning ctx to ctxPool.
	return ctxPool.Get().(*Context)
}

func (ctx *Context) prepare() {
	if ctx.UseGlobalCache {
		ctx.cache = globalCache
	} else {
		ctx.cache = make(simpleCache, 8)
	}

	ctx.flags = 0
	if ctx.CopyBetweenPtrAndValue {
		ctx.flags |= 1 << flagBitCopyBetweenPtrAndValue
	}
	if ctx.IgnoreNonCopyableTypes {
		ctx.flags |= 1 << flagBitIgnoreNonCopyableTypes
	}
	if ctx.SkipZeroFields {
		ctx.flags |= 1 << flagBitSkipZeroFields
	}
}

func (ctx *Context) keyFor(dstType, srcType reflect.Type) planKey {
	return planKey{dstType: dstType, srcType: srcType, flags: ctx.flags}
}

// resolvePlan selects or builds the appropriate Plan for (dstType, srcType).
// It delegates to focused sub-functions for each source kind to keep each
// decision path short and independently readable.
func resolvePlan(ctx *Context, dstType, srcType reflect.Type) (Plan, error) {
	// ① User-registered zero-reflection copier — highest priority.
	// Registered functions bypass the plan cache and the entire reflection engine.
	if p, ok := customRegistry.Load(customKey{dstType: dstType, srcType: srcType}); ok {
		return p.(Plan), nil
	}

	key := ctx.keyFor(dstType, srcType)
	cached, found := ctx.cache.get(key)
	if cached != nil {
		return cached, nil
	}

	dstKind, srcKind := dstType.Kind(), srcType.Kind()

	// Scalars and funcs — no caching needed (stateless singletons).
	if scalarKinds&(1<<srcKind) > 0 {
		return resolveScalarPlan(ctx, dstType, srcType)
	}

	// Channel: share reference.
	if srcKind == reflect.Chan && dstKind == reflect.Chan {
		return defaultChanPlan, nil
	}

	// Interface dst / src — wrap with a runtime-dispatch plan.
	if dstKind == reflect.Interface {
		return resolveIfaceDstPlan(ctx, key, dstType, srcType)
	}
	if srcKind == reflect.Interface {
		return resolveIfaceSrcPlan(ctx, key, dstType, srcType)
	}

	// Pointer kinds.
	if srcKind == reflect.Pointer || dstKind == reflect.Pointer {
		return resolvePointerPlan(ctx, key, dstType, srcType, dstKind, srcKind)
	}

	// Slice / Array.
	if srcKind == reflect.Slice || srcKind == reflect.Array {
		return resolveSlicePlan(ctx, key, dstType, srcType, dstKind)
	}

	// Struct source.
	if srcKind == reflect.Struct {
		return resolveStructSrcPlan(ctx, key, dstType, srcType, dstKind, found)
	}

	// Map source.
	if srcKind == reflect.Map {
		return resolveMapSrcPlan(ctx, key, dstType, srcType, dstKind)
	}

	return nonCopyable(ctx, dstType, srcType)
}

// resolveScalarPlan handles scalar and func source types.
func resolveScalarPlan(ctx *Context, dstType, srcType reflect.Type) (Plan, error) {
	if dstType == srcType {
		return defaultAssignPlan, nil
	}
	if srcType.ConvertibleTo(dstType) {
		return defaultConvertPlan, nil
	}
	return nonCopyable(ctx, dstType, srcType)
}

// resolveIfaceDstPlan handles an interface destination type.
func resolveIfaceDstPlan(ctx *Context, key planKey, dstType, srcType reflect.Type) (Plan, error) {
	p := &ifaceDstPlan{}
	if err := p.init(ctx, dstType, srcType); err != nil {
		return nil, err
	}
	ctx.cache.set(key, p)
	return p, nil
}

// resolveIfaceSrcPlan handles an interface source type.
func resolveIfaceSrcPlan(ctx *Context, key planKey, dstType, srcType reflect.Type) (Plan, error) {
	p := &ifaceSrcPlan{}
	if err := p.init(ctx, dstType, srcType); err != nil {
		return nil, err
	}
	ctx.cache.set(key, p)
	return p, nil
}

// resolvePointerPlan handles all combinations of pointer src/dst.
func resolvePointerPlan(ctx *Context, key planKey, dstType, srcType reflect.Type, dstKind, srcKind reflect.Kind) (Plan, error) {
	if srcKind == reflect.Pointer {
		if dstKind == reflect.Pointer {
			p := &ptrPlan{}
			if err := p.init(ctx, dstType, srcType); err != nil {
				return nil, err
			}
			ctx.cache.set(key, p)
			return p, nil
		}
		if ctx.CopyBetweenPtrAndValue {
			p := &derefPlan{}
			if err := p.init(ctx, dstType, srcType); err != nil {
				return nil, err
			}
			ctx.cache.set(key, p)
			return p, nil
		}
		return nonCopyable(ctx, dstType, srcType)
	}
	// dstKind == reflect.Pointer, srcKind != reflect.Pointer
	if ctx.CopyBetweenPtrAndValue {
		p := &addrPlan{}
		if err := p.init(ctx, dstType, srcType); err != nil {
			return nil, err
		}
		ctx.cache.set(key, p)
		return p, nil
	}
	return nonCopyable(ctx, dstType, srcType)
}

// resolveSlicePlan handles slice and array source types.
func resolveSlicePlan(ctx *Context, key planKey, dstType, srcType reflect.Type, dstKind reflect.Kind) (Plan, error) {
	if dstKind != reflect.Slice && dstKind != reflect.Array {
		return nonCopyable(ctx, dstType, srcType)
	}
	p := &slicePlan{}
	if err := p.init(ctx, dstType, srcType); err != nil {
		return nil, err
	}
	ctx.cache.set(key, p)
	return p, nil
}

// resolveStructSrcPlan handles a struct source type copying to a struct or map dst.
func resolveStructSrcPlan(ctx *Context, key planKey, dstType, srcType reflect.Type, dstKind reflect.Kind, found bool) (Plan, error) {
	if dstKind == reflect.Struct {
		// Fast path: same type, every field is flat → a single Set is a correct deep copy.
		// Flat types have no heap-allocated fields (no slices, maps, pointers, interfaces,
		// channels), so Set is equivalent to a manual field-by-field assignment and avoids
		// N per-field reflect calls entirely.
		//
		// Skip this optimisation when per-field evaluation is required:
		//   • SkipZeroFields (Merge) needs to check each source field for zero-ness.
		//   • fieldMask (WithFields) needs to skip fields not in the mask.
		if dstType == srcType && isFlatType(srcType) &&
			ctx.flags&(1<<flagBitSkipZeroFields) == 0 &&
			ctx.fieldMask == nil {
			ctx.cache.set(key, defaultAssignPlan)
			return defaultAssignPlan, nil
		}

		// Circular reference guard: nil sentinel signals in-progress.
		if found {
			return &deferredPlan{dstType: dstType, srcType: srcType}, nil
		}
		ctx.cache.set(key, nil)

		p := &structPlan{}
		if err := p.init(ctx, dstType, srcType); err != nil {
			ctx.cache.del(key)
			return nil, err
		}
		ctx.cache.set(key, p)
		return p, nil
	}
	if dstKind == reflect.Map {
		p := &structToMapPlan{}
		if err := p.init(ctx, dstType, srcType); err != nil {
			return nil, err
		}
		ctx.cache.set(key, p)
		return p, nil
	}
	return nonCopyable(ctx, dstType, srcType)
}

// resolveMapSrcPlan handles a map source type copying to a map or struct dst.
func resolveMapSrcPlan(ctx *Context, key planKey, dstType, srcType reflect.Type, dstKind reflect.Kind) (Plan, error) {
	if dstKind == reflect.Map {
		p := &mapPlan{}
		if err := p.init(ctx, dstType, srcType); err != nil {
			return nil, err
		}
		ctx.cache.set(key, p)
		return p, nil
	}
	if dstKind == reflect.Struct {
		p := &mapToStructPlan{}
		if err := p.init(ctx, dstType, srcType); err != nil {
			return nil, err
		}
		ctx.cache.set(key, p)
		return p, nil
	}
	return nonCopyable(ctx, dstType, srcType)
}

func nonCopyable(ctx *Context, dstType, srcType reflect.Type) (Plan, error) {
	if ctx.IgnoreNonCopyableTypes {
		return defaultSkipPlan, nil
	}
	return nil, newCopyError(ErrTypeNonCopyable, srcType, dstType)
}
