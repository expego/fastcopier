package fastcopier

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

// ctxPool reuses Context objects to reduce per-call allocations.
var ctxPool = sync.Pool{
	New: func() any {
		return &Context{}
	},
}

// mergeCtxPool reuses Context objects for Merge (SkipZeroFields=true) calls.
var mergeCtxPool = sync.Pool{
	New: func() any {
		return &Context{}
	},
}

const (
	// DefaultTagName is the struct tag key used to configure field copying.
	DefaultTagName = "fastcopier"
)

var defaultTagNameAtomic atomic.Value

func init() {
	defaultTagNameAtomic.Store(DefaultTagName)
}

// CircularRefPolicy controls what happens when a pointer cycle is detected during copying.
type CircularRefPolicy int

const (
	// CircularRefError returns ErrCircularReference when a cycle is detected (default).
	CircularRefError CircularRefPolicy = iota
	// CircularRefSkip silently sets the back-pointer to nil instead of returning an error.
	CircularRefSkip
)

// Context holds configuration for a single copy operation.
type Context struct {
	// CopyBetweenPtrAndValue allows copying between pointers and values (default: true).
	CopyBetweenPtrAndValue bool

	// IgnoreNonCopyableTypes silently skips non-copyable types instead of returning an error (default: false).
	IgnoreNonCopyableTypes bool

	// UseGlobalCache controls whether the global plan cache is used (default: true).
	UseGlobalCache bool

	// CircularRef controls behaviour when a pointer cycle is detected (default: CircularRefError).
	CircularRef CircularRefPolicy

	// SkipZeroFields skips copying fields whose source value is the zero value for its type.
	// When true, only non-zero source fields overwrite the destination — useful for PATCH semantics.
	// Used by Merge; can also be set directly via WithSkipZeroFields.
	SkipZeroFields bool

	// TagName overrides the struct tag key used for field configuration (default: "fastcopier").
	// When empty, the global defaultTagName is used.
	TagName string

	// fieldMask, when non-nil, restricts copying to only the named fields (matched by key after tag resolution).
	fieldMask map[string]struct{}

	cache planCache

	// visited tracks pointer addresses for circular-reference detection (lazy-initialised).
	visited map[uintptr]bool

	flags uint8
}

// Option is a configuration function for Copy.
type Option func(ctx *Context)

// WithCopyBetweenPtrAndValue sets whether copying between pointers and values is allowed.
func WithCopyBetweenPtrAndValue(flag bool) Option {
	return func(ctx *Context) { ctx.CopyBetweenPtrAndValue = flag }
}

// WithIgnoreNonCopyableTypes sets whether non-copyable types are silently skipped.
func WithIgnoreNonCopyableTypes(flag bool) Option {
	return func(ctx *Context) { ctx.IgnoreNonCopyableTypes = flag }
}

// WithGlobalCache sets whether the global plan cache is used.
func WithGlobalCache(flag bool) Option {
	return func(ctx *Context) { ctx.UseGlobalCache = flag }
}

// Copy performs a deep copy from src into dst.
// dst must be a non-nil pointer to a struct, slice, or map; src may be a value or pointer.
// For the common []Src → []Dst conversion, see Map.
func Copy(dst, src any, options ...Option) error {
	dstVal, srcVal, dstType, err := validateAndDeref(dst, src)
	if err != nil {
		return err
	}

	var ctx *Context
	if len(options) == 0 {
		ctx = ctxPool.Get().(*Context)
	} else {
		ctx = ctxPool.Get().(*Context)
		for _, opt := range options {
			opt(ctx)
		}
	}
	ctx.prepare()

	plan, err := resolvePlan(ctx, dstType, srcVal.Type())
	if err != nil {
		ctx.reset()
		ctxPool.Put(ctx)
		return err
	}
	err = plan.Copy(dstVal, srcVal, ctx)
	ctx.reset()
	ctxPool.Put(ctx)
	return err
}

// validateAndDeref performs the pre-flight checks shared by Copy and Merge:
//   - both dst and src must be non-nil
//   - dst must be a non-nil pointer
//   - src must be non-nil (if a pointer)
//   - src is dereferenced once so callers always receive a non-pointer srcVal
//
// On success it returns the dereferenced dst value, the dereferenced src value,
// and the element type of dst. On failure it returns a descriptive wrapped error.
func validateAndDeref(dst, src any) (dstVal, srcVal reflect.Value, dstType reflect.Type, err error) {
	if dst == nil || src == nil {
		err = fmt.Errorf("%w: source and destination must be non-nil", ErrValueInvalid)
		return
	}
	dstVal = reflect.ValueOf(dst)
	srcVal = reflect.ValueOf(src)
	dstType = dstVal.Type()
	if dstType.Kind() != reflect.Pointer {
		err = fmt.Errorf("%w: destination must be a pointer", ErrTypeInvalid)
		return
	}
	dstVal = dstVal.Elem()
	dstType = dstType.Elem()
	if !dstVal.IsValid() {
		err = fmt.Errorf("%w: destination must be non-nil", ErrValueInvalid)
		return
	}
	if srcVal.Kind() == reflect.Pointer && srcVal.IsNil() {
		err = fmt.Errorf("%w: source must be non-nil", ErrValueInvalid)
		return
	}
	if srcVal.Kind() == reflect.Pointer {
		srcVal = srcVal.Elem()
	}
	return
}

// reset clears per-call state so the Context can be returned to the pool.
// It resets ALL fields to their defaults so that no stale state leaks between calls.
func (ctx *Context) reset() {
	for k := range ctx.visited {
		delete(ctx.visited, k)
	}
	ctx.fieldMask = nil
	ctx.TagName = ""
	ctx.SkipZeroFields = false
	ctx.IgnoreNonCopyableTypes = false
	ctx.CircularRef = CircularRefError
	ctx.CopyBetweenPtrAndValue = true
	ctx.UseGlobalCache = true
	ctx.flags = 0
	ctx.cache = nil
}

// effectiveTagName returns the tag name to use for this context.
func (ctx *Context) effectiveTagName() string {
	if ctx.TagName != "" {
		return ctx.TagName
	}
	return defaultTagNameAtomic.Load().(string)
}

// WithCircularReferencePolicy sets the behaviour when a pointer cycle is detected.
func WithCircularReferencePolicy(p CircularRefPolicy) Option {
	return func(ctx *Context) { ctx.CircularRef = p }
}

// WithTagName overrides the struct tag key used for field configuration for this copy operation.
// When not set, the global defaultTagName ("fastcopier") is used.
// Using a custom tag name bypasses the global plan cache to avoid cross-contamination.
func WithTagName(tag string) Option {
	return func(ctx *Context) {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			ctx.TagName = tag
			ctx.UseGlobalCache = false
		}
	}
}

// WithFields restricts copying to only the named fields (matched by key after tag resolution).
//
// WARNING: WithFields bypasses the global plan cache on every call. Do NOT use it in
// hot loops — each call rebuilds the field-filtered plan from scratch. For repeated
// use with the same field set, call MustRegister (or Inspect) once at startup and
// then use Copy without options; the cached plan already covers only the fields you
// registered.
func WithFields(fields ...string) Option {
	return func(ctx *Context) {
		if len(fields) == 0 {
			return
		}
		ctx.fieldMask = make(map[string]struct{}, len(fields))
		for _, f := range fields {
			ctx.fieldMask[f] = struct{}{}
		}
		ctx.UseGlobalCache = false
	}
}

// WithSkipZeroFields instructs Copy to skip any source field whose value is the zero
// value for its type. Only non-zero fields overwrite the destination.
// This is the PATCH semantics option — see also Merge.
func WithSkipZeroFields(skip bool) Option {
	return func(ctx *Context) { ctx.SkipZeroFields = skip }
}

// Clone returns a deep copy of src.
// It is a generic convenience wrapper around Copy that eliminates the need to
// pre-declare a destination variable and pass its address.
func Clone[T any](src T, options ...Option) (T, error) {
	var dst T
	if err := Copy(&dst, &src, options...); err != nil {
		return dst, err
	}
	return dst, nil
}

// Map converts a []Src slice into a []Dst slice by copying each element.
// It is a generic convenience wrapper around Copy for the common []Src → []Dst
// pattern, eliminating the need to pre-declare the destination slice.
//
// Note: passing options disables the global plan cache for this call (a fresh plan is
// built each time). For hot-loop use, call MustRegister at startup and omit options.
//
// Example:
//
//	dtos, err := fastcopier.Map[UserEntity, UserDTO](entities)
func Map[Src, Dst any](src []Src, options ...Option) ([]Dst, error) {
	var dst []Dst
	if err := Copy(&dst, &src, options...); err != nil {
		return nil, err
	}
	return dst, nil
}

// Merge copies only the non-zero fields of src into dst, leaving existing dst
// values intact where the corresponding src field is zero.
//
// This implements PATCH semantics: only fields explicitly set in src overwrite dst.
// It is equivalent to Copy with WithSkipZeroFields(true).
//
// Example:
//
//	existing := User{Name: "Alice", Age: 30, Email: "alice@example.com"}
//	patch := User{Email: "new@example.com"} // only Email is non-zero
//	fastcopier.Merge(&existing, &patch)
//	// existing.Name == "Alice", existing.Age == 30, existing.Email == "new@example.com"
func Merge(dst, src any, options ...Option) error {
	if len(options) == 0 {
		// Fast path: use a dedicated pool for SkipZeroFields=true contexts,
		// avoiding the []Option allocation that WithSkipZeroFields(true) would cause.
		dstVal, srcVal, dstType, err := validateAndDeref(dst, src)
		if err != nil {
			return err
		}

		ctx := mergeCtxPool.Get().(*Context)
		ctx.SkipZeroFields = true
		ctx.prepare()

		plan, err := resolvePlan(ctx, dstType, srcVal.Type())
		if err != nil {
			ctx.reset()
			mergeCtxPool.Put(ctx)
			return err
		}
		err = plan.Copy(dstVal, srcVal, ctx)
		ctx.reset()
		mergeCtxPool.Put(ctx)
		return err
	}
	opts := make([]Option, 0, len(options)+1)
	opts = append(opts, WithSkipZeroFields(true))
	opts = append(opts, options...)
	return Copy(dst, src, opts...)
}

// SetDefaultTagName changes the struct tag key used for field configuration.
// Should be called once at program startup before any Copy calls.
//
// Stale-cache warning: if called after Copy calls have already been made, call
// ClearCache() immediately afterwards to discard plans built with the old tag.
func SetDefaultTagName(tag string) {
	tag = strings.TrimSpace(tag)
	if tag != "" {
		defaultTagNameAtomic.Store(tag)
	}
}

// MustRegister pre-builds and validates the copy plan for the given (dst, src) type
// pair, panicking if the plan cannot be constructed. Call this in init() or main()
// to catch field mapping errors at startup rather than at runtime.
//
// It is a no-op if the plan is already cached from a previous call.
//
// Example:
//
//	func init() {
//	    fastcopier.MustRegister(&UserDTO{}, &UserEntity{})
//	    fastcopier.MustRegister(&OrderDTO{}, &OrderEntity{})
//	}
func MustRegister(dst, src any, options ...Option) {
	_, err := Inspect(dst, src, options...)
	if err != nil {
		panic(fmt.Sprintf("fastcopier.MustRegister: %v", err))
	}
}

// MustRegisterWithFields validates that a copy plan restricted to the named fields
// can be constructed for the given (dst, src) type pair, panicking if it cannot.
// Call this in init() or main() to catch field mapping errors for partial-copy
// operations at startup rather than at runtime.
//
// Note: because WithFields bypasses the global plan cache, callers still pay
// plan-build cost on each Copy call that uses WithFields. For hot paths, consider
// pre-registering the full plan with MustRegister and avoiding WithFields at runtime.
//
// Example:
//
//	func init() {
//	    fastcopier.MustRegisterWithFields(&UserDTO{}, &UserEntity{}, "Name", "Email")
//	}
func MustRegisterWithFields(dst, src any, fields ...string) {
	_, err := Inspect(dst, src, WithFields(fields...))
	if err != nil {
		panic(fmt.Sprintf("fastcopier.MustRegisterWithFields: %v", err))
	}
}

// ClearCache discards all cached copy plans. Useful in tests to force plan
// rebuilds after changing options or tag names. Not needed in production.
func ClearCache() {
	globalCache.clear()
}
