package fastcopier_test

import (
	"errors"
	"testing"

	"github.com/expego/fastcopier"
)

// ── 5.1: CopyError.Error() with field context ─────────────────────────────────

func TestCopyError_Error_WithField(t *testing.T) {
	ce := &fastcopier.CopyError{
		SrcType:  "SrcStruct",
		DstType:  "DstStruct",
		SrcField: "Name",
		DstField: "Name",
		Err:      fastcopier.ErrTypeNonCopyable,
	}
	msg := ce.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	if !errors.Is(ce, fastcopier.ErrTypeNonCopyable) {
		t.Fatal("errors.Is should unwrap to ErrTypeNonCopyable")
	}
}

func TestCopyError_Error_NoField(t *testing.T) {
	ce := &fastcopier.CopyError{
		SrcType: "int",
		DstType: "string",
		Err:     fastcopier.ErrTypeNonCopyable,
	}
	msg := ce.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

// ── 5.2: reset() clears all fields (pool safety) ─────────────────────────────

func TestReset_PoolSafety(t *testing.T) {
	// First call with non-default options; second call must see defaults.
	type S struct{ X int }
	src1 := S{X: 42}
	var dst1 S
	if err := fastcopier.Copy(&dst1, &src1, fastcopier.WithSkipZeroFields(true)); err != nil {
		t.Fatal(err)
	}

	// Second call with no options — should not inherit SkipZeroFields from first.
	src2 := S{X: 0}
	dst2 := S{X: 99}
	if err := fastcopier.Copy(&dst2, &src2); err != nil {
		t.Fatal(err)
	}
	if dst2.X != 0 {
		t.Fatalf("expected dst2.X=0 (SkipZeroFields must not leak), got %d", dst2.X)
	}
}

// ── 5.3: MustRegisterWithFields ───────────────────────────────────────────────

func TestMustRegisterWithFields_OK(t *testing.T) {
	type Src struct{ Name, Email string }
	type Dst struct{ Name, Email string }
	// Should not panic.
	fastcopier.MustRegisterWithFields(&Dst{}, &Src{}, "Name")
}

func TestMustRegisterWithFields_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	// nil dst should panic
	fastcopier.MustRegisterWithFields(nil, &struct{}{})
}

// ── 5.4: Merge with extra options ─────────────────────────────────────────────

func TestMerge_WithOptions(t *testing.T) {
	type S struct{ Name, Email string }
	existing := S{Name: "Alice", Email: "alice@example.com"}
	patch := S{Email: "new@example.com"}
	if err := fastcopier.Merge(&existing, &patch, fastcopier.WithCopyBetweenPtrAndValue(true)); err != nil {
		t.Fatal(err)
	}
	if existing.Name != "Alice" {
		t.Fatalf("Name should be preserved, got %q", existing.Name)
	}
	if existing.Email != "new@example.com" {
		t.Fatalf("Email should be updated, got %q", existing.Email)
	}
}

// ── 5.5: Clone nil pointer ────────────────────────────────────────────────────

func TestClone_NilPointer(t *testing.T) {
	// Cloning a nil pointer returns a nil pointer with no error.
	var p *struct{ X int }
	result, err := fastcopier.Clone(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

// ── 5.6: Map error path ───────────────────────────────────────────────────────

func TestMap_Error(t *testing.T) {
	type Src struct{ Ch chan int }
	type Dst struct{ Ch chan int }
	src := []Src{{Ch: make(chan int)}}
	// chan fields are copyable (chanPlan), so use a truly incompatible type.
	type DstBad struct{ Ch string }
	_, err := fastcopier.Map[Src, DstBad](src)
	// chan → string is non-copyable; expect error
	if err == nil {
		t.Fatal("expected error mapping chan to string")
	}
}

// ── 5.7: fieldByIndexInit (embedded nil pointer initialisation) ───────────────

func TestFieldByIndexInit_NilEmbedded(t *testing.T) {
	type Inner struct{ Value int }
	type Outer struct {
		*Inner
		Name string
	}
	src := Outer{Inner: &Inner{Value: 7}, Name: "test"}
	var dst Outer
	if err := fastcopier.Copy(&dst, &src); err != nil {
		t.Fatal(err)
	}
	if dst.Inner == nil {
		t.Fatal("expected Inner to be initialised")
	}
	if dst.Inner.Value != 7 {
		t.Fatalf("expected Value=7, got %d", dst.Inner.Value)
	}
}

// ── 5.8: fieldSetZero (nil embedded pointer in src) ───────────────────────────

func TestFieldSetZero_NilEmbedded(t *testing.T) {
	type Inner struct{ Value int }
	type Outer struct {
		*Inner
		Name string
	}
	// src has nil Inner — dst.Inner.Value should be zeroed.
	src := Outer{Inner: nil, Name: "test"}
	dst := Outer{Inner: &Inner{Value: 99}, Name: "old"}
	if err := fastcopier.Copy(&dst, &src); err != nil {
		t.Fatal(err)
	}
	// Inner is nil in src; dst.Inner.Value should be zeroed (fieldSetZero path).
	if dst.Inner != nil && dst.Inner.Value != 0 {
		t.Fatalf("expected Value=0 after nil-src embedded, got %d", dst.Inner.Value)
	}
}

// ── 5.9: CircularRefSkip policy ───────────────────────────────────────────────

func TestCircularRefSkip(t *testing.T) {
	type Node struct {
		Val  int
		Next *Node
	}
	n := &Node{Val: 1}
	n.Next = n // cycle

	var dst Node
	err := fastcopier.Copy(&dst, n, fastcopier.WithCircularReferencePolicy(fastcopier.CircularRefSkip))
	if err != nil {
		t.Fatalf("expected no error with CircularRefSkip, got %v", err)
	}
	// The first level copies fine; the back-pointer (cycle) is set to nil.
	if dst.Val != 1 {
		t.Fatalf("expected Val=1, got %d", dst.Val)
	}
	if dst.Next != nil && dst.Next.Next != nil {
		t.Fatal("expected cycle to be broken (back-pointer nil)")
	}
}

// ── 5.10: Inspect for slice, map, and pointer top-level types ─────────────────

func TestInspect_SliceTopLevel(t *testing.T) {
	type Item struct{ X int }
	plan, err := fastcopier.Inspect(&[]Item{}, &[]Item{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Fields) == 0 {
		t.Fatal("expected at least one field mapping for slice")
	}
}

func TestInspect_MapTopLevel(t *testing.T) {
	plan, err := fastcopier.Inspect(&map[string]int{}, &map[string]int{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Fields) == 0 {
		t.Fatal("expected at least one field mapping for map")
	}
}

func TestInspect_PtrTopLevel(t *testing.T) {
	type S struct{ X int }
	src := &S{X: 1}
	dst := &S{}
	plan, err := fastcopier.Inspect(&dst, &src)
	if err != nil {
		t.Fatal(err)
	}
	_ = plan
}
