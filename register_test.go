package fastcopier

import (
	"errors"
	"reflect"
	"testing"
)

// ── RegisterCopier tests ──────────────────────────────────────────────────────

type regSrc struct{ X, Y int }
type regDst struct{ X, Y int }

// TestRegisterCopier_CalledInsteadOfReflection verifies that a registered
// function is called (not the reflection engine) for the matching type pair.
func TestRegisterCopier_CalledInsteadOfReflection(t *testing.T) {
	called := false
	RegisterCopier(func(dst *regDst, src *regSrc) error {
		called = true
		dst.X = src.X * 10 // intentionally different from a plain copy
		dst.Y = src.Y * 10
		return nil
	})
	defer customRegistry.Delete(customKey{
		dstType: reflectTypeOf[regDst](),
		srcType: reflectTypeOf[regSrc](),
	})

	src := regSrc{X: 3, Y: 7}
	var dst regDst
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if !called {
		t.Error("registered function was not called")
	}
	if dst.X != 30 || dst.Y != 70 {
		t.Errorf("expected {30,70}, got {%d,%d}", dst.X, dst.Y)
	}
}

// TestRegisterCopier_FallbackWhenNotRegistered confirms that types WITHOUT a
// registered function still use the reflection engine correctly.
func TestRegisterCopier_FallbackWhenNotRegistered(t *testing.T) {
	type noReg struct{ V string }
	src := noReg{V: "hello"}
	var dst noReg
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst.V != "hello" {
		t.Errorf("expected hello, got %q", dst.V)
	}
}

// TestRegisterCopier_ErrorPropagated verifies that errors returned by the
// registered function are surfaced by fastcopier.Copy.
func TestRegisterCopier_ErrorPropagated(t *testing.T) {
	type errSrc struct{ N int }
	type errDst struct{ N int }
	sentinel := errors.New("copy refused")

	RegisterCopier(func(dst *errDst, src *errSrc) error {
		return sentinel
	})
	defer customRegistry.Delete(customKey{
		dstType: reflectTypeOf[errDst](),
		srcType: reflectTypeOf[errSrc](),
	})

	src := errSrc{N: 1}
	var dst errDst
	err := Copy(&dst, &src)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

// TestRegisterCopier_OverwritesPreviousRegistration ensures the last Register
// call for a type pair wins.
func TestRegisterCopier_OverwritesPreviousRegistration(t *testing.T) {
	type ovSrc struct{ V int }
	type ovDst struct{ V int }

	RegisterCopier(func(dst *ovDst, src *ovSrc) error { dst.V = 1; return nil })
	RegisterCopier(func(dst *ovDst, src *ovSrc) error { dst.V = 2; return nil })
	defer customRegistry.Delete(customKey{
		dstType: reflectTypeOf[ovDst](),
		srcType: reflectTypeOf[ovSrc](),
	})

	src := ovSrc{V: 99}
	var dst ovDst
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst.V != 2 {
		t.Errorf("expected 2 (second registration), got %d", dst.V)
	}
}

// TestRegisterCopier_SrcPassedAsValue exercises the non-addressable src path.
func TestRegisterCopier_SrcPassedAsValue(t *testing.T) {
	type valSrc struct{ N int }
	type valDst struct{ N int }
	RegisterCopier(func(dst *valDst, src *valSrc) error {
		dst.N = src.N + 1
		return nil
	})
	defer customRegistry.Delete(customKey{
		dstType: reflectTypeOf[valDst](),
		srcType: reflectTypeOf[valSrc](),
	})

	// Pass src as value (non-pointer) — exercises the Interface() fallback.
	src := valSrc{N: 5}
	var dst valDst
	if err := Copy(&dst, src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst.N != 6 {
		t.Errorf("expected 6, got %d", dst.N)
	}
}

// BenchmarkRegisteredCopier measures the overhead of the registered-function path.
func BenchmarkRegisteredCopier(b *testing.B) {
	type bSrc struct{ A, B, C int }
	type bDst struct{ A, B, C int }
	RegisterCopier(func(dst *bDst, src *bSrc) error {
		dst.A = src.A
		dst.B = src.B
		dst.C = src.C
		return nil
	})
	defer customRegistry.Delete(customKey{
		dstType: reflectTypeOf[bDst](),
		srcType: reflectTypeOf[bSrc](),
	})

	src := bSrc{A: 1, B: 2, C: 3}
	var dst bDst
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Copy(&dst, &src)
	}
}

// reflectTypeOf is a small helper to get reflect.Type without an instance.
func reflectTypeOf[T any]() reflect.Type {
	var zero T
	return reflect.TypeOf(zero)
}
