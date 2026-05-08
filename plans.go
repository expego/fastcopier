package fastcopier

import (
	"fmt"
	"reflect"
)

// Plan executes a copy from src to dst using the provided per-call Context.
type Plan interface {
	Copy(dst, src reflect.Value, ctx *Context) error
}

// ── Leaf plans ────────────────────────────────────────────────────────────────

// skipPlan is a no-op that silently ignores non-copyable fields.
type skipPlan struct{}

func (skipPlan) Copy(_, _ reflect.Value, _ *Context) error { return nil }

var defaultSkipPlan = skipPlan{}

// assignPlan copies src directly to dst (identical types).
type assignPlan struct{}

func (assignPlan) Copy(dst, src reflect.Value, _ *Context) error { dst.Set(src); return nil }

var defaultAssignPlan = assignPlan{}

// convertPlan copies src to dst with a type conversion.
type convertPlan struct{}

func (convertPlan) Copy(dst, src reflect.Value, _ *Context) error {
	dst.Set(src.Convert(dst.Type()))
	return nil
}

var defaultConvertPlan = convertPlan{}

// chanPlan copies a channel by reference.
type chanPlan struct{}

func (chanPlan) Copy(dst, src reflect.Value, _ *Context) error { dst.Set(src); return nil }

var defaultChanPlan = chanPlan{}

// ── Circular reference helper ─────────────────────────────────────────────────

// checkCircularRef checks for pointer cycles.
// Returns (skip=true) when the caller should set dst to zero and return nil.
// Returns a non-nil error when a cycle is detected and the policy is CircularRefError.
// When skip==false and err==nil, the caller must defer delete(ctx.visited, src.Pointer()).
//
// Note on nil-map safety: reading from a nil map in Go always returns the zero value
// (false for bool) without panicking, so ctx.visited[ptr] is safe even before the map
// is initialised. The map is only written to after the read, so lazy initialisation here
// is correct and race-free (Context is never shared across goroutines).
func checkCircularRef(src reflect.Value, ctx *Context) (skip bool, err error) {
	if src.IsNil() {
		return false, nil
	}
	ptr := src.Pointer()
	// Safe even when ctx.visited == nil: nil map reads return the zero value in Go.
	if ctx.visited[ptr] {
		if ctx.CircularRef == CircularRefSkip {
			return true, nil
		}
		return false, fmt.Errorf("%w", ErrCircularReference)
	}
	if ctx.visited == nil {
		ctx.visited = make(map[uintptr]bool, 4)
	}
	ctx.visited[ptr] = true
	return false, nil
}

// ── Pointer plans ─────────────────────────────────────────────────────────────

// ptrPlan copies *T → *T, allocating the destination if nil.
type ptrPlan struct {
	elem Plan
}

func (p *ptrPlan) Copy(dst, src reflect.Value, ctx *Context) error {
	skip, err := checkCircularRef(src, ctx)
	if err != nil {
		return err
	}
	if skip {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	if !src.IsNil() {
		defer delete(ctx.visited, src.Pointer())
	}
	src = src.Elem()
	if !src.IsValid() {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	if dst.IsNil() {
		dst.Set(reflect.New(dst.Type().Elem()))
	}
	return p.elem.Copy(dst.Elem(), src, ctx)
}

func (p *ptrPlan) init(ctx *Context, dstType, srcType reflect.Type) (err error) {
	p.elem, err = resolvePlan(ctx, dstType.Elem(), srcType.Elem())
	return
}

// derefPlan copies *T → V (dereferences src pointer).
type derefPlan struct {
	elem Plan
}

func (p *derefPlan) Copy(dst, src reflect.Value, ctx *Context) error {
	skip, err := checkCircularRef(src, ctx)
	if err != nil {
		return err
	}
	if skip {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	if !src.IsNil() {
		defer delete(ctx.visited, src.Pointer())
	}
	src = src.Elem()
	if !src.IsValid() {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	return p.elem.Copy(dst, src, ctx)
}

func (p *derefPlan) init(ctx *Context, dstType, srcType reflect.Type) (err error) {
	p.elem, err = resolvePlan(ctx, dstType, srcType.Elem())
	return
}

// addrPlan copies V → *T (allocates destination if nil).
type addrPlan struct {
	elem Plan
}

func (p *addrPlan) Copy(dst, src reflect.Value, ctx *Context) error {
	if dst.IsNil() {
		dst.Set(reflect.New(dst.Type().Elem()))
	}
	return p.elem.Copy(dst.Elem(), src, ctx)
}

func (p *addrPlan) init(ctx *Context, dstType, srcType reflect.Type) (err error) {
	p.elem, err = resolvePlan(ctx, dstType.Elem(), srcType)
	return
}

// deferredPlan breaks circular plan-build references by looking up the real plan at copy time.
// It cannot cache the result because ctx is per-call and returned to a pool after each Copy.
// The lookup is a single sharded cache read (O(1)) so the overhead is minimal.
type deferredPlan struct {
	dstType reflect.Type
	srcType reflect.Type
}

func (p *deferredPlan) Copy(dst, src reflect.Value, ctx *Context) error {
	real, err := resolvePlan(ctx, p.dstType, p.srcType)
	if err != nil {
		return err
	}
	return real.Copy(dst, src, ctx)
}

// ── Interface plans ───────────────────────────────────────────────────────────

// ifaceSrcPlan copies from an interface src to a concrete dst.
type ifaceSrcPlan struct{}

func (p *ifaceSrcPlan) init(_ *Context, _, _ reflect.Type) error { return nil }

func (p *ifaceSrcPlan) Copy(dst, src reflect.Value, ctx *Context) error {
	for src.Kind() == reflect.Interface {
		src = src.Elem()
		if !src.IsValid() {
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}
	}
	plan, err := resolvePlan(ctx, dst.Type(), src.Type())
	if err != nil {
		return err
	}
	return plan.Copy(dst, src, ctx)
}

// ifaceDstPlan copies a concrete src into an interface dst (clones the value first).
type ifaceDstPlan struct{}

func (p *ifaceDstPlan) init(_ *Context, _, _ reflect.Type) error { return nil }

func (p *ifaceDstPlan) Copy(dst, src reflect.Value, ctx *Context) error {
	for src.Kind() == reflect.Interface {
		src = src.Elem()
		if !src.IsValid() {
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}
	}
	srcType := src.Type()
	cloned := reflect.New(srcType).Elem()
	plan, err := resolvePlan(ctx, srcType, srcType)
	if err != nil {
		return err
	}
	if err = plan.Copy(cloned, src, ctx); err != nil {
		return err
	}
	dst.Set(cloned)
	return nil
}
