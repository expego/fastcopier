package fastcopier

// Tests that exercise code paths previously at 0% or very low coverage.
// Each test is documented with which internal path it targets.
//
// Additional tests are in copier_inspect_gaps_test.go (external package).

import (
	"errors"
	"testing"
)

// ── fieldByIndexInit ─────────────────────────────────────────────────────────
// Triggered in fieldPlan.Copy when len(p.dstIndex) > 1 (struct dst field reached
// through anonymous embedding: e.g. EmbedDst.Inner.X has dstIndex=[0,0]).

func TestFieldByIndexInit_StructToAnonymousEmbedDst(t *testing.T) {
	type Inner struct{ X int }
	type EmbedDst struct{ Inner } // X promoted, dstIndex=[0,0]
	type Src struct{ X int }

	src := Src{X: 42}
	var dst EmbedDst
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.X != 42 {
		t.Errorf("X = %d, want 42", dst.X)
	}
}

func TestFieldByIndexInit_DeepAnonymousEmbedDst(t *testing.T) {
	// Two levels of anonymous embedding → dstIndex=[0,0,0]
	type Leaf struct{ V int }
	type Mid struct{ Leaf } // promoted Leaf
	type Top struct{ Mid }  // promoted Mid (and transitively Leaf)
	type Src struct{ V int }

	src := Src{V: 99}
	var dst Top
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.V != 99 {
		t.Errorf("V = %d, want 99", dst.V)
	}
}

// ── fieldSetZero ─────────────────────────────────────────────────────────────
// Triggered in fieldPlan.Copy when len(p.srcIndex) > 1 AND FieldByIndexErr fails
// (nil anonymous embedded pointer src). The dst field is zeroed.

func TestFieldSetZero_NilAnonymousEmbedPtrSrc(t *testing.T) {
	type Inner struct{ X int }
	type SrcEmbed struct{ *Inner } // anonymous pointer embed; nil → srcIndex=[0,0] fails
	type Dst struct{ X int }

	src := SrcEmbed{Inner: nil}
	dst := Dst{X: 99}
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// dst.X must be zeroed because the source path (through nil *Inner) failed
	if dst.X != 0 {
		t.Errorf("X = %d, want 0 (field should be zeroed when src path is nil)", dst.X)
	}
}

// ── ptrPlan.Copy — circular-reference skip path ──────────────────────────────
// Covered by a self-referential struct copied with CircularRefSkip policy.

func TestPtrPlan_CircularRefSkip(t *testing.T) {
	type Node struct {
		Value int
		Next  *Node
	}

	n := &Node{Value: 1}
	n.Next = n // cycle

	var dst Node
	err := Copy(&dst, n, WithCircularReferencePolicy(CircularRefSkip))
	if err != nil {
		t.Fatalf("unexpected error with CircularRefSkip: %v", err)
	}
	if dst.Value != 1 {
		t.Errorf("Value = %d, want 1", dst.Value)
	}
	// The first level is copied (dst.Next is allocated).
	if dst.Next == nil {
		t.Fatal("Next should be non-nil (first-level copy)")
	}
	// The back-pointer (second level, which is the cycle) is set to nil.
	if dst.Next.Next != nil {
		t.Error("Next.Next should be nil (circular back-pointer skipped)")
	}
}

// ── derefPlan.Copy — nil-src path ────────────────────────────────────────────
// When the source field is a *T pointer but the value is nil, derefPlan sets dst to zero.

func TestDerefPlan_NilPointerSrc(t *testing.T) {
	type Inner struct{ X int }
	type Src struct{ Inner *Inner }
	type Dst struct{ Inner Inner }

	src := Src{Inner: nil}
	dst := Dst{Inner: Inner{X: 77}}
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Inner should be zeroed because source pointer was nil
	if dst.Inner.X != 0 {
		t.Errorf("Inner.X = %d, want 0 (dst field should be zero when src ptr is nil)", dst.Inner.X)
	}
}

// ── val2FieldPlan.Copy — deep dstIndex (map→struct with anonymous embed) ─────
// Triggered when len(p.dstIndex) > 1 in val2FieldPlan.Copy, which calls fieldByIndexInit.

func TestVal2FieldPlan_DeepDstIndex_MapToEmbeddedStruct(t *testing.T) {
	type Inner struct{ X int }
	type EmbedDst struct{ Inner } // X promoted, dstIndex=[0,0]

	m := map[string]int{"X": 55}
	var dst EmbedDst
	if err := Copy(&dst, &m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.X != 55 {
		t.Errorf("X = %d, want 55", dst.X)
	}
}

// ── val2FieldPlan.Copy — elem=nil direct-assign path ─────────────────────────
// When map value type == dst field type exactly, buildEntryPlan returns elem=nil
// and val2FieldPlan.Copy uses dst.Set(src) directly.

func TestVal2FieldPlan_DirectScalarAssign(t *testing.T) {
	type Dst struct{ Count int }

	// int value type = int field type → elem=nil → dst.Set(src) path
	m := map[string]int{"Count": 13}
	var dst Dst
	if err := Copy(&dst, &m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Count != 13 {
		t.Errorf("Count = %d, want 13", dst.Count)
	}
}

// ── mapToStructPlan.Copy — required field tracking ───────────────────────────

func TestMapToStruct_RequiredField_Present_NoError(t *testing.T) {
	type Dst struct {
		Name  string
		Email string `fastcopier:",required"`
	}
	m := map[string]string{"Name": "Alice", "Email": "alice@example.com"}
	var dst Dst
	if err := Copy(&dst, &m); err != nil {
		t.Fatalf("unexpected error for required-present: %v", err)
	}
	if dst.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", dst.Email, "alice@example.com")
	}
}

func TestMapToStruct_RequiredField_Missing_Error(t *testing.T) {
	// Clear the cache so the plan is freshly built with the required field.
	ClearCache()
	type Dst struct {
		Name  string
		Email string `fastcopier:",required"`
	}
	m := map[string]string{"Name": "Alice"} // no "Email"
	var dst Dst
	err := Copy(&dst, &m)
	if err == nil {
		t.Fatal("expected ErrFieldRequireCopying, got nil")
	}
	if !errors.Is(err, ErrFieldRequireCopying) {
		t.Errorf("expected ErrFieldRequireCopying, got: %v", err)
	}
}

// ── field2MapPlan.Copy — nil anonymous embedded pointer src (srcIndex len > 1) ─
// Triggered when struct→map copies a field through an anonymous embedded pointer
// that is nil; the field is silently skipped (not added to the map).

func TestField2MapPlan_NilAnonEmbedPtrSrc(t *testing.T) {
	type Inner struct{ X int }
	type SrcEmbed struct{ *Inner } // anonymous embedded pointer; X at srcIndex=[0,0]

	src := SrcEmbed{Inner: nil}
	dst := map[string]int{}
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// X must not be present in the map since the src path was nil
	if _, ok := dst["X"]; ok {
		t.Error("expected key 'X' to be absent from dst map (src path was nil)")
	}
}

// ── structPlan.buildFieldPlan — IgnoreNonCopyableTypes + required ─────────────

func TestBuildFieldPlan_IgnoreNonCopyable_RequiredDst_Error(t *testing.T) {
	// When IgnoreNonCopyableTypes=true, resolvePlan returns skipPlan for incompatible
	// types. But if the DST field is also required, that skipPlan must be rejected.
	type Src struct{ X func() }
	type Dst struct {
		X chan int `fastcopier:",required"`
	}
	ClearCache()
	err := Copy(&Dst{}, &Src{}, WithIgnoreNonCopyableTypes(true))
	if err == nil {
		t.Fatal("expected error: required dst field got a skip plan")
	}
	if !errors.Is(err, ErrFieldRequireCopying) {
		t.Errorf("expected ErrFieldRequireCopying, got: %v", err)
	}
}

func TestBuildFieldPlan_IgnoreNonCopyable_RequiredSrc_Error(t *testing.T) {
	// Same as above but required flag is on the SRC field.
	type Src struct {
		X func() `fastcopier:",required"`
	}
	type Dst struct{ X chan int }
	ClearCache()
	err := Copy(&Dst{}, &Src{}, WithIgnoreNonCopyableTypes(true))
	if err == nil {
		t.Fatal("expected error: required src field got a skip plan")
	}
	if !errors.Is(err, ErrFieldRequireCopying) {
		t.Errorf("expected ErrFieldRequireCopying, got: %v", err)
	}
}

// ── fieldByIndexInit — nil pointer in path (lines A-D of fieldByIndexInit) ───
// Two levels of anonymous embedding where the intermediate level is a *pointer.
// When the ptr is nil, fieldByIndexInit allocates it (covers the IsNil branch).

func TestFieldByIndexInit_NilPointerInPath(t *testing.T) {
	type L2 struct{ X int }
	type L1 struct{ *L2 } // anonymous embedded pointer: *L2 may be nil
	type Top struct{ L1 } // anonymous embedded struct
	type Src struct{ X int }

	src := Src{X: 77}
	var dst Top // dst.L1.L2 is nil — fieldByIndexInit must allocate it
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.L2 == nil {
		t.Fatal("L2 should have been allocated by fieldByIndexInit")
	}
	if dst.X != 77 {
		t.Errorf("X = %d, want 77", dst.X)
	}
}

// ── buildEntryPlan — scalar-convert path (map→struct with int → int64) ────────
// When the map value type is scalar and *convertible* (but not identical) to the
// dst field type, buildEntryPlan returns a val2FieldPlan wrapping convertPlan.

func TestBuildEntryPlan_ScalarConvert_MapToStruct(t *testing.T) {
	type Dst struct{ Count int64 }
	// map value type = int, dst field type = int64 → scalar-convert path
	m := map[string]int{"Count": 42}
	var dst Dst
	if err := Copy(&dst, &m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Count != 42 {
		t.Errorf("Count = %d, want 42", dst.Count)
	}
}

// ── structToMapPlan.buildFieldPlan — needConvert path ────────────────────────
// When the dst map key type is not plain string but string-convertible (e.g.
// type MyKey string), buildFieldPlan converts the key with mapKey.Convert.

func TestStructToMap_CustomStringKey(t *testing.T) {
	type MyKey = string // defined type convertible to string
	type Src struct{ Name string }

	src := Src{Name: "hello"}
	dst := map[MyKey]string{}
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst["Name"] != "hello" {
		t.Errorf("Name = %q, want %q", dst["Name"], "hello")
	}
}

// ── structToMapPlan.buildFieldPlan — scalar-convert path ────────────────────
// When src struct field type is scalar and convertible to map value type.

func TestStructToMap_ScalarConvert_FieldToMapVal(t *testing.T) {
	type Src struct{ Count int32 }
	// map value type = int64; int32 is convertible → scalar-convert path
	src := Src{Count: 7}
	dst := map[string]int64{}
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst["Count"] != 7 {
		t.Errorf("Count = %d, want 7", dst["Count"])
	}
}

// ── Clone — error path (fastcopier.go:232-234) ───────────────────────────────
// Clone wraps Copy; when Copy returns an error it must forward it to the caller.
// A self-referential struct with the default CircularRefError policy triggers this.

func TestClone_ErrorPath_CircularRef(t *testing.T) {
	type Node struct {
		V    int
		Next *Node
	}
	n := &Node{V: 1}
	n.Next = n // cycle: n.Next → n

	// Clone(*n) → Copy(&dst, &src) where src.Next still points to n → circular ref error
	_, err := Clone(*n)
	if err == nil {
		t.Fatal("expected error for circular reference in Clone, got nil")
	}
	if !errors.Is(err, ErrCircularReference) {
		t.Errorf("expected ErrCircularReference, got %v", err)
	}
}

// ── Copy — nil dst pointer (fastcopier.go:113-115) ────────────────────────────
// When dst is a non-nil interface wrapping a nil pointer, dstVal.Elem() is
// invalid and Copy must return ErrValueInvalid.

func TestCopy_NilDstPointer(t *testing.T) {
	type S struct{ X int }
	var nilPtr *S // non-nil interface, nil pointer
	src := S{X: 1}
	err := Copy(nilPtr, &src)
	if err == nil {
		t.Fatal("expected error for nil dst pointer, got nil")
	}
	if !errors.Is(err, ErrValueInvalid) {
		t.Errorf("expected ErrValueInvalid, got %v", err)
	}
}

// ── ifaceSrcPlan.Copy — nil interface src (plans.go:178-181) ─────────────────
// When an interface{} source field is nil, ifaceSrcPlan must zero the dst field.

func TestIfaceSrc_NilInterface(t *testing.T) {
	type SrcIface struct{ V any }
	type DstConcrete struct{ V int }

	src := SrcIface{V: nil}
	dst := DstConcrete{V: 99}
	ClearCache()
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.V != 0 {
		t.Errorf("V = %d, want 0 (nil interface should zero dst field)", dst.V)
	}
}

// ── ifaceSrcPlan.Copy — resolvePlan error (plans.go:184-186) ─────────────────
// When the concrete type inside interface{} is incompatible with dst, Copy errors.

func TestIfaceSrc_IncompatibleConcreteType(t *testing.T) {
	type SrcIface struct{ V any }
	type DstConcrete struct{ V int }

	src := SrcIface{V: make(chan int)} // chan int → int is non-copyable
	var dst DstConcrete
	ClearCache()
	err := Copy(&dst, &src)
	if err == nil {
		t.Fatal("expected error for chan→int copy via interface, got nil")
	}
}

// ── simpleCache.del — triggered on structPlan.init failure (engine.go:109) ────
// With UseGlobalCache=false, ctx.cache is a simpleCache.  When structPlan.init
// fails, ctx.cache.del must remove the in-progress nil sentinel.

func TestSimpleCache_Del_OnStructPlanFailure(t *testing.T) {
	type SrcBadField struct{ X func() }
	type DstBadField struct{ X int }

	ClearCache()
	err := Copy(&DstBadField{}, &SrcBadField{}, WithGlobalCache(false))
	if err == nil {
		t.Fatal("expected error for func→int field copy, got nil")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected ErrTypeNonCopyable, got %v", err)
	}
}

// ── resolvePlan scalar ConvertibleTo path (engine.go:232-234) ─────────────────
// resolvePlan's scalar branch returns defaultConvertPlan when srcType != dstType
// but srcType.ConvertibleTo(dstType).  Triggered via slicePlan.init where the
// slice element types differ (e.g. []int32 → []int64).

func TestResolvePlan_Slice_DifferentScalarElements(t *testing.T) {
	src := []int32{10, 20, 30}
	var dst []int64
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dst) != 3 || dst[0] != 10 || dst[1] != 20 || dst[2] != 30 {
		t.Errorf("unexpected result: %v", dst)
	}
}

// ── resolvePlan nonCopyable — Slice src, non-Slice/Array dst (engine.go:297-299) ─
// When a src field is a slice and dst field is a plain scalar, resolvePlan hits
// the "slice to non-slice is non-copyable" branch.

func TestResolvePlan_SliceToNonSlice_Error(t *testing.T) {
	type SrcSliceField struct{ X []int }
	type DstScalarField struct{ X string }

	ClearCache()
	err := Copy(&DstScalarField{}, &SrcSliceField{X: []int{1, 2}})
	if err == nil {
		t.Fatal("expected error copying []int to string, got nil")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected ErrTypeNonCopyable, got %v", err)
	}
}

// ── resolvePlan nonCopyable — Struct src, non-Struct/Map dst (engine.go:348) ──
// When a src field is a struct and dst field is a scalar, resolvePlan returns
// nonCopyable from the struct branch.

func TestResolvePlan_StructToScalar_Error(t *testing.T) {
	type Inner struct{ V int }
	type SrcStructField struct{ X Inner }
	type DstScalarField struct{ X int }

	ClearCache()
	err := Copy(&DstScalarField{}, &SrcStructField{X: Inner{V: 7}})
	if err == nil {
		t.Fatal("expected error copying struct to int, got nil")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected ErrTypeNonCopyable, got %v", err)
	}
}

// ── resolvePlan nonCopyable — Map src, non-Map/Struct dst (engine.go:369) ─────
// When a src field is a map and dst field is a scalar, resolvePlan returns
// nonCopyable from the map branch.

func TestResolvePlan_MapToScalar_Error(t *testing.T) {
	type SrcMapField struct{ X map[string]int }
	type DstScalarField struct{ X int }

	ClearCache()
	err := Copy(&DstScalarField{}, &SrcMapField{X: map[string]int{"k": 1}})
	if err == nil {
		t.Fatal("expected error copying map to int, got nil")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected ErrTypeNonCopyable, got %v", err)
	}
}

// ── resolvePlan nonCopyable — dst is Ptr, CopyBetweenPtrAndValue=false (engine.go:292) ─
// When dst field is *T but src field is chan/map/slice (non-pointer non-scalar),
// and CopyBetweenPtrAndValue=false, resolvePlan hits the nonCopyable branch
// for the value→pointer case.

func TestResolvePlan_ValueToPtr_NoCopyBetweenPtrAndValue_Error(t *testing.T) {
	type SrcChanField struct{ X chan int }
	type DstPtrField struct{ X *int }

	ClearCache()
	err := Copy(&DstPtrField{}, &SrcChanField{X: make(chan int)},
		WithCopyBetweenPtrAndValue(false))
	if err == nil {
		t.Fatal("expected error copying chan→*int with CopyBetweenPtrAndValue=false")
	}
}

// ── resolvePlan ptrPlan.init error (engine.go:267-269) ───────────────────────
// ptrPlan.init calls resolvePlan for the elem types.  If the inner types are
// incompatible (*chan int → *int), ptrPlan.init returns an error.

func TestResolvePlan_PtrPlanInitError(t *testing.T) {
	type SrcPtrChan struct{ F *chan int }
	type DstPtrInt struct{ F *int }

	chanVal := make(chan int)
	ClearCache()
	err := Copy(&DstPtrInt{}, &SrcPtrChan{F: &chanVal})
	if err == nil {
		t.Fatal("expected error copying *chan int → *int, got nil")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected ErrTypeNonCopyable, got %v", err)
	}
}

// ── resolvePlan derefPlan.init error (engine.go:275-277) ─────────────────────
// derefPlan.init calls resolvePlan(ctx, dstType, srcType.Elem()).
// When *chan int → int is requested, the inner resolvePlan(int, chan int) fails.

func TestResolvePlan_DerefPlanInitError(t *testing.T) {
	// dst field = int (value), src field = *chan int (pointer).
	// derefPlan init: resolvePlan(int, chan int) → nonCopyable → error.
	type SrcPtrChan struct{ F *chan int }
	type DstInt struct{ F int }

	chanVal := make(chan int)
	ClearCache()
	err := Copy(&DstInt{}, &SrcPtrChan{F: &chanVal})
	if err == nil {
		t.Fatal("expected error copying *chan int → int, got nil")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected ErrTypeNonCopyable, got %v", err)
	}
}

// ── resolvePlan addrPlan.init error (engine.go:286-288) ──────────────────────
// addrPlan.init calls resolvePlan(ctx, dstType.Elem(), srcType).
// When chan int → *int is requested, the inner resolvePlan(int, chan int) fails.

func TestResolvePlan_AddrPlanInitError(t *testing.T) {
	// dst field = *int (pointer), src field = chan int (non-pointer non-scalar).
	// addrPlan init: resolvePlan(int, chan int) → nonCopyable → error.
	type SrcChan struct{ F chan int }
	type DstPtrInt struct{ F *int }

	ClearCache()
	err := Copy(&DstPtrInt{}, &SrcChan{F: make(chan int)})
	if err == nil {
		t.Fatal("expected error copying chan int → *int, got nil")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected ErrTypeNonCopyable, got %v", err)
	}
}

// ── resolvePlan mapPlan.init error (engine.go:355-357) ───────────────────────
// mapPlan.init calls resolvePlan for value types.  When the value types are
// incompatible (chan int → string), mapPlan.init returns an error.

func TestResolvePlan_MapPlanInitError(t *testing.T) {
	src := map[string]chan int{"k": make(chan int)}
	dst := map[string]string{}
	ClearCache()
	err := Copy(&dst, &src)
	if err == nil {
		t.Fatal("expected error copying map[string]chan int → map[string]string, got nil")
	}
}

// ── ifaceDstPlan.Copy — plan.Copy error path (plans.go:209-211) ──────────────
// When an interface{} src field holds a *Node with a self-referential cycle,
// ifaceDstPlan.Copy tries to clone the *Node and plan.Copy returns
// ErrCircularReference, which must be forwarded.

func TestIfaceDstPlan_PlanCopyError_CircularRef(t *testing.T) {
	type Node struct {
		V    int
		Next *Node
	}
	type WithIface struct{ V any }

	n := &Node{V: 5}
	n.Next = n // cycle

	src := WithIface{V: n}
	var dst WithIface
	ClearCache()
	err := Copy(&dst, &src)
	if err == nil {
		t.Fatal("expected error from circular ref inside interface clone, got nil")
	}
	if !errors.Is(err, ErrCircularReference) {
		t.Errorf("expected ErrCircularReference, got %v", err)
	}
}
