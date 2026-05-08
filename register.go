package fastcopier

import (
	"reflect"
	"sync"
)

// customKey identifies a registered copier by its (dst, src) type pair.
type customKey struct {
	dstType reflect.Type
	srcType reflect.Type
}

// customRegistry maps (dstType, srcType) pairs to user-registered Plans.
// Entries are written once (at init time) and read on every Copy call, so
// sync.Map is the right tool: read-heavy, write-once, concurrent-safe.
var customRegistry sync.Map // map[customKey]Plan

// RegisterCopier registers a zero-reflection copy function for Dst←Src copies.
//
// When fastcopier.Copy(&dst, &src) is called and the concrete types of dst
// and src match Dst and Src exactly, fn is called directly — the reflection
// engine is bypassed entirely.
//
// Typical usage is in an init() function inside a file produced by
// fastcopier-gen:
//
//	func init() {
//	    fastcopier.RegisterCopier(CopyUserEntityToUserDTO)
//	}
//
// RegisterCopier is safe for concurrent use. Registering the same type pair
// twice overwrites the previous entry.
func RegisterCopier[Dst, Src any](fn func(dst *Dst, src *Src) error) {
	var zeroDst Dst
	var zeroSrc Src
	dstType := reflect.TypeOf(&zeroDst).Elem()
	srcType := reflect.TypeOf(&zeroSrc).Elem()
	p := &registeredFuncPlan[Dst, Src]{fn: fn}
	customRegistry.Store(customKey{dstType: dstType, srcType: srcType}, Plan(p))
}

// registeredFuncPlan wraps a user-provided copy function as a Plan.
// It uses unsafe.Pointer for the hot path (zero allocation) and falls back
// to reflect.Value.Interface() when src is not addressable (rare: the caller
// passed a non-pointer value as src).
type registeredFuncPlan[Dst, Src any] struct {
	fn func(*Dst, *Src) error
}

// Copy implements Plan. ctx is unused because registered functions manage
// their own copy logic and do not need circular-reference tracking via the
// fastcopier Context (they are expected to be cycle-free or handle it themselves).
func (p *registeredFuncPlan[Dst, Src]) Copy(dst, src reflect.Value, _ *Context) error {
	// dst is always addressable: it comes from reflect.ValueOf(dstPtr).Elem().
	dstPtr := (*Dst)(dst.Addr().UnsafePointer())

	// src is addressable when the caller passed a pointer (the common case,
	// because Copy() dereferences the src pointer upfront in fastcopier.go).
	if src.CanAddr() {
		return p.fn(dstPtr, (*Src)(src.Addr().UnsafePointer()))
	}

	// Rare: src was passed as a bare value (not a pointer).
	// Box it once to get an addressable copy, then call fn.
	v := src.Interface().(Src)
	return p.fn(dstPtr, &v)
}
