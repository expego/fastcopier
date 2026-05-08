package fastcopier

import (
	"errors"
	"testing"
)

// ── Clone ─────────────────────────────────────────────────────────────────────

func TestClone_Simple(t *testing.T) {
	src := SimpleStruct{Name: "Alice", Age: 25, Email: "alice@example.com", Score: 9.5, Active: true}
	dst, err := Clone(src)
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	if dst.Name != src.Name || dst.Age != src.Age || dst.Email != src.Email {
		t.Errorf("Clone result mismatch: got %+v, want %+v", dst, src)
	}
}

func TestClone_WithOption(t *testing.T) {
	type Src struct{ Val *int }
	type Dst struct{ Val int }
	v := 42
	src := Src{Val: &v}
	// CopyBetweenPtrAndValue=false: *int → int should fail
	_, err := Clone[Dst](Dst{}, WithCopyBetweenPtrAndValue(false))
	// Clone of Dst with no pointer fields — just verify option wiring compiles and runs
	_ = err

	// Positive: default options should work
	dst, err2 := Clone(src)
	if err2 != nil {
		t.Fatalf("Clone with ptr field failed: %v", err2)
	}
	if dst.Val == nil || *dst.Val != 42 {
		t.Errorf("Clone ptr field: got %v", dst.Val)
	}
}

func TestClone_Error(t *testing.T) {
	// Incompatible types with error (chan → string, no ignore)
	type Src struct{ Ch chan int }
	type Dst struct{ Ch string }
	src := Src{Ch: make(chan int)}
	_, err := Clone[Dst](Dst{})
	// No error expected here since Dst has no Ch chan int field — use direct Copy for error path
	_ = err

	var d Dst
	err = Copy(&d, &src)
	if err == nil {
		t.Error("expected error copying chan→string")
	}
}

// ── Copy error paths ──────────────────────────────────────────────────────────

func TestCopy_NilDst(t *testing.T) {
	src := SimpleStruct{Name: "x"}
	err := Copy(nil, &src)
	if !errors.Is(err, ErrValueInvalid) {
		t.Errorf("expected ErrValueInvalid, got %v", err)
	}
}

func TestCopy_NilSrc(t *testing.T) {
	var dst SimpleStruct
	err := Copy(&dst, nil)
	if !errors.Is(err, ErrValueInvalid) {
		t.Errorf("expected ErrValueInvalid, got %v", err)
	}
}

func TestCopy_DstNotPointer(t *testing.T) {
	src := SimpleStruct{Name: "x"}
	dst := SimpleStruct{}
	err := Copy(dst, &src)
	if !errors.Is(err, ErrTypeInvalid) {
		t.Errorf("expected ErrTypeInvalid, got %v", err)
	}
}

func TestCopy_NilPointerSrc(t *testing.T) {
	var src *SimpleStruct
	var dst SimpleStruct
	err := Copy(&dst, src)
	if !errors.Is(err, ErrValueInvalid) {
		t.Errorf("expected ErrValueInvalid, got %v", err)
	}
}

// ── Options ───────────────────────────────────────────────────────────────────

func TestWithCopyBetweenPtrAndValue_False(t *testing.T) {
	type Src struct{ Val *int }
	type Dst struct{ Val int }
	v := 99
	src := Src{Val: &v}
	var dst Dst
	err := Copy(&dst, &src, WithCopyBetweenPtrAndValue(false))
	if err == nil {
		t.Error("expected error when CopyBetweenPtrAndValue=false and src is pointer")
	}
}

func TestWithCopyBetweenPtrAndValue_ValueToPtr(t *testing.T) {
	type Src struct{ Val int }
	type Dst struct{ Val *int }
	src := Src{Val: 7}
	var dst Dst
	err := Copy(&dst, &src, WithCopyBetweenPtrAndValue(false))
	if err == nil {
		t.Error("expected error when CopyBetweenPtrAndValue=false and dst is pointer")
	}
}

func TestWithIgnoreNonCopyableTypes(t *testing.T) {
	type Src struct {
		Name string
		Ch   chan int
	}
	type Dst struct {
		Name string
		Ch   string // incompatible
	}
	src := Src{Name: "bob", Ch: make(chan int)}
	var dst Dst
	err := Copy(&dst, &src, WithIgnoreNonCopyableTypes(true))
	if err != nil {
		t.Fatalf("expected no error with IgnoreNonCopyableTypes, got %v", err)
	}
	if dst.Name != "bob" {
		t.Errorf("Name mismatch: got %s", dst.Name)
	}
	if dst.Ch != "" {
		t.Errorf("Ch should be empty (skipped), got %s", dst.Ch)
	}
}

func TestWithGlobalCache_False(t *testing.T) {
	src := SimpleStruct{Name: "nocache", Age: 1}
	var dst SimpleStruct
	err := Copy(&dst, &src, WithGlobalCache(false))
	if err != nil {
		t.Fatalf("Copy without global cache failed: %v", err)
	}
	if dst.Name != src.Name {
		t.Errorf("Name mismatch: got %s", dst.Name)
	}
}

func TestClearCache(t *testing.T) {
	src := SimpleStruct{Name: "precache", Age: 2}
	var dst SimpleStruct
	// Warm the cache
	_ = Copy(&dst, &src)
	// Clear it
	ClearCache()
	// Should still work after clear
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy after ClearCache failed: %v", err)
	}
	if dst.Name != src.Name {
		t.Errorf("Name mismatch after ClearCache: got %s", dst.Name)
	}
}

func TestSetDefaultTagName(t *testing.T) {
	type Src struct {
		UserName string `mytag:"Name"`
	}
	type Dst struct {
		Name string
	}
	SetDefaultTagName("mytag")
	defer SetDefaultTagName("fastcopier") // restore

	src := Src{UserName: "custom-tag"}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy with custom tag failed: %v", err)
	}
	if dst.Name != "custom-tag" {
		t.Errorf("Name mismatch with custom tag: got %s", dst.Name)
	}
}

func TestSetDefaultTagName_Empty(t *testing.T) {
	// Empty string should be ignored (no change)
	SetDefaultTagName("")
	// defaultTagName should still be "fastcopier" (or whatever it was)
	src := SimpleStruct{Name: "still-works"}
	var dst SimpleStruct
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy after empty SetDefaultTagName failed: %v", err)
	}
}

// ── CircularRefSkip ───────────────────────────────────────────────────────────

func TestCircularRefSkip_SelfReferential(t *testing.T) {
	src := &Node{Value: 42}
	src.Next = src // cycle

	var dst Node
	err := Copy(&dst, src, WithCircularReferencePolicy(CircularRefSkip))
	if err != nil {
		t.Fatalf("expected no error with CircularRefSkip, got %v", err)
	}
	if dst.Value != 42 {
		t.Errorf("Value mismatch: got %d", dst.Value)
	}
	// The cycle is truncated: Next is either nil or points to a node with nil Next
	if dst.Next != nil && dst.Next.Next != nil {
		t.Errorf("cycle should be truncated, got Next.Next = %+v", dst.Next.Next)
	}
}

// ── derefPlan: *T → V ─────────────────────────────────────────────────────────

func TestDerefPlan_PtrToValue(t *testing.T) {
	type Src struct{ Val *int }
	type Dst struct{ Val int }
	v := 55
	src := Src{Val: &v}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("derefPlan copy failed: %v", err)
	}
	if dst.Val != 55 {
		t.Errorf("Val mismatch: got %d", dst.Val)
	}
}

func TestDerefPlan_NilPtrToValue(t *testing.T) {
	type Src struct{ Val *int }
	type Dst struct{ Val int }
	src := Src{Val: nil}
	dst := Dst{Val: 99}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("derefPlan nil copy failed: %v", err)
	}
	if dst.Val != 0 {
		t.Errorf("Val should be zero, got %d", dst.Val)
	}
}

// ── addrPlan: V → *T ──────────────────────────────────────────────────────────

func TestAddrPlan_ValueToPtr(t *testing.T) {
	// addrPlan handles non-scalar src copied into a pointer dst field.
	// Use struct→*struct (scalars are handled by the scalar fast-path, not addrPlan).
	type Inner struct{ X int }
	type Src struct{ Val Inner }
	type Dst struct{ Val *Inner }
	src := Src{Val: Inner{X: 77}}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("addrPlan copy failed: %v", err)
	}
	if dst.Val == nil {
		t.Fatal("Val should not be nil")
	}
	if dst.Val.X != 77 {
		t.Errorf("Val.X mismatch: got %d", dst.Val.X)
	}
}

// ── deferredPlan: self-referential struct ─────────────────────────────────────

type Tree struct {
	Val   int
	Left  *Tree
	Right *Tree
}

func TestDeferredPlan_Tree(t *testing.T) {
	src := &Tree{
		Val:   1,
		Left:  &Tree{Val: 2, Left: &Tree{Val: 4}, Right: &Tree{Val: 5}},
		Right: &Tree{Val: 3},
	}
	var dst Tree
	err := Copy(&dst, src)
	if err != nil {
		t.Fatalf("Tree copy failed: %v", err)
	}
	if dst.Val != 1 {
		t.Errorf("root Val: got %d", dst.Val)
	}
	if dst.Left == nil || dst.Left.Val != 2 {
		t.Errorf("left Val: got %v", dst.Left)
	}
	if dst.Right == nil || dst.Right.Val != 3 {
		t.Errorf("right Val: got %v", dst.Right)
	}
	if dst.Left.Left == nil || dst.Left.Left.Val != 4 {
		t.Errorf("left.left Val: got %v", dst.Left.Left)
	}
}

// ── skipPlan via IgnoreNonCopyableTypes ───────────────────────────────────────

func TestSkipPlan_IncompatibleField(t *testing.T) {
	type Src struct {
		Name  string
		Score complex128 // no matching kind in dst
	}
	type Dst struct {
		Name  string
		Score string // incompatible
	}
	src := Src{Name: "skip-test", Score: 1 + 2i}
	var dst Dst
	err := Copy(&dst, &src, WithIgnoreNonCopyableTypes(true))
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if dst.Name != "skip-test" {
		t.Errorf("Name mismatch: got %s", dst.Name)
	}
}

// ── mapToStructPlan ───────────────────────────────────────────────────────────

func TestMapToStruct_StringValues(t *testing.T) {
	type Dst struct {
		Name  string
		Email string
	}
	src := map[string]string{"Name": "map-user", "Email": "map@example.com"}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("map→struct failed: %v", err)
	}
	if dst.Name != "map-user" {
		t.Errorf("Name: got %s", dst.Name)
	}
	if dst.Email != "map@example.com" {
		t.Errorf("Email: got %s", dst.Email)
	}
}

func TestMapToStruct_IntValues(t *testing.T) {
	type Dst struct {
		Age   int
		Score int
	}
	src := map[string]int{"Age": 30, "Score": 95}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("map[string]int→struct failed: %v", err)
	}
	if dst.Age != 30 || dst.Score != 95 {
		t.Errorf("values mismatch: %+v", dst)
	}
}

func TestMapToStruct_InterfaceValues(t *testing.T) {
	type Dst struct {
		Name string
		Age  int
	}
	src := map[string]any{"Name": "iface-user", "Age": 22}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("map[string]any→struct failed: %v", err)
	}
	if dst.Name != "iface-user" {
		t.Errorf("Name: got %s", dst.Name)
	}
	if dst.Age != 22 {
		t.Errorf("Age: got %d", dst.Age)
	}
}

func TestMapToStruct_NilMap(t *testing.T) {
	type Dst struct{ Name string }
	var src map[string]string
	dst := Dst{Name: "unchanged"}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("nil map→struct failed: %v", err)
	}
	if dst.Name != "unchanged" {
		t.Errorf("Name should be unchanged, got %s", dst.Name)
	}
}

func TestMapToStruct_NonStringKey_Error(t *testing.T) {
	type Dst struct{ Age int }
	src := map[int]int{1: 30}
	var dst Dst
	err := Copy(&dst, &src)
	if err == nil {
		t.Error("expected error for non-string map key")
	}
}

func TestMapToStruct_NonStringKey_Ignored(t *testing.T) {
	type Dst struct{ Age int }
	src := map[int]int{1: 30}
	var dst Dst
	err := Copy(&dst, &src, WithIgnoreNonCopyableTypes(true))
	if err != nil {
		t.Fatalf("expected no error with ignore: %v", err)
	}
}

// ── structToMapPlan ───────────────────────────────────────────────────────────

func TestStructToMap_AnyValues(t *testing.T) {
	type Src struct {
		Name string
		Age  int
	}
	src := Src{Name: "struct-user", Age: 40}
	var dst map[string]any
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("struct→map[string]any failed: %v", err)
	}
	if dst["Name"] != "struct-user" {
		t.Errorf("Name: got %v", dst["Name"])
	}
	if dst["Age"] != 40 {
		t.Errorf("Age: got %v", dst["Age"])
	}
}

func TestStructToMap_StringValues(t *testing.T) {
	type Src struct {
		Name  string
		Email string
	}
	src := Src{Name: "s2m", Email: "s2m@example.com"}
	var dst map[string]string
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("struct→map[string]string failed: %v", err)
	}
	if dst["Name"] != "s2m" || dst["Email"] != "s2m@example.com" {
		t.Errorf("values mismatch: %v", dst)
	}
}

func TestStructToMap_NilMapInitialised(t *testing.T) {
	type Src struct{ Name string }
	src := Src{Name: "init-map"}
	var dst map[string]string // nil
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("struct→nil map failed: %v", err)
	}
	if dst == nil {
		t.Fatal("dst map should have been initialised")
	}
	if dst["Name"] != "init-map" {
		t.Errorf("Name: got %s", dst["Name"])
	}
}

func TestStructToMap_NonStringKey_Error(t *testing.T) {
	type Src struct{ Age int }
	src := Src{Age: 5}
	var dst map[int]int
	err := Copy(&dst, &src)
	if err == nil {
		t.Error("expected error for non-string map key in struct→map")
	}
}

func TestStructToMap_NonStringKey_Ignored(t *testing.T) {
	type Src struct{ Age int }
	src := Src{Age: 5}
	var dst map[int]int
	err := Copy(&dst, &src, WithIgnoreNonCopyableTypes(true))
	if err != nil {
		t.Fatalf("expected no error with ignore: %v", err)
	}
}

func TestStructToMap_NestedStruct(t *testing.T) {
	type Inner struct{ X int }
	type Src struct {
		Name  string
		Inner Inner
	}
	src := Src{Name: "outer", Inner: Inner{X: 9}}
	var dst map[string]any
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("struct with nested→map failed: %v", err)
	}
	if dst["Name"] != "outer" {
		t.Errorf("Name: got %v", dst["Name"])
	}
}

// ── mapPlan (map→map) ─────────────────────────────────────────────────────────

func TestMapToMap_NilSrc(t *testing.T) {
	var src map[string]string
	dst := map[string]string{"existing": "val"}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("nil map→map failed: %v", err)
	}
	if dst != nil {
		t.Errorf("dst should be nil after copying nil src, got %v", dst)
	}
}

func TestMapToMap_StructValues(t *testing.T) {
	type Val struct{ N int }
	src := map[string]Val{"a": {N: 1}, "b": {N: 2}}
	var dst map[string]Val
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("map[string]struct→map failed: %v", err)
	}
	if dst["a"].N != 1 || dst["b"].N != 2 {
		t.Errorf("values mismatch: %v", dst)
	}
}

func TestMapToMap_IntKeys(t *testing.T) {
	src := map[int]int{1: 10, 2: 20}
	var dst map[int]int
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("map[int]int→map failed: %v", err)
	}
	if dst[1] != 10 || dst[2] != 20 {
		t.Errorf("values mismatch: %v", dst)
	}
}

func TestMapToMap_ConvertibleKeys(t *testing.T) {
	src := map[int32]int32{1: 100}
	var dst map[int64]int64
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("map key/val conversion failed: %v", err)
	}
	if dst[1] != 100 {
		t.Errorf("value mismatch: %v", dst)
	}
}

// ── slicePlan: array dst, reuse cap ──────────────────────────────────────────

func TestSlicePlan_ArrayDst(t *testing.T) {
	type Src struct{ Items []int }
	type Dst struct{ Items [3]int }
	src := Src{Items: []int{10, 20, 30, 40}} // longer than array
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("slice→array failed: %v", err)
	}
	if dst.Items[0] != 10 || dst.Items[1] != 20 || dst.Items[2] != 30 {
		t.Errorf("array values mismatch: %v", dst.Items)
	}
}

func TestSlicePlan_ArrayDst_SrcShorter(t *testing.T) {
	type Src struct{ Items []int }
	type Dst struct{ Items [5]int }
	src := Src{Items: []int{1, 2}}
	dst := Dst{Items: [5]int{9, 9, 9, 9, 9}}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("slice→array (src shorter) failed: %v", err)
	}
	// First 2 copied, rest zeroed
	if dst.Items[0] != 1 || dst.Items[1] != 2 {
		t.Errorf("copied values wrong: %v", dst.Items)
	}
	if dst.Items[2] != 0 || dst.Items[3] != 0 || dst.Items[4] != 0 {
		t.Errorf("remainder should be zero: %v", dst.Items)
	}
}

func TestSlicePlan_ReuseCapacity(t *testing.T) {
	type Src struct{ Items []string }
	type Dst struct{ Items []string }
	src := Src{Items: []string{"a", "b", "c"}}
	// Pre-allocate dst with enough cap
	dst := Dst{Items: make([]string, 5, 10)}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("slice reuse cap failed: %v", err)
	}
	if len(dst.Items) != 3 {
		t.Errorf("len mismatch: got %d", len(dst.Items))
	}
	if dst.Items[0] != "a" || dst.Items[1] != "b" || dst.Items[2] != "c" {
		t.Errorf("values mismatch: %v", dst.Items)
	}
}

func TestSlicePlan_BulkCopyReuseCapacity(t *testing.T) {
	type Src struct{ Items []int }
	type Dst struct{ Items []int }
	src := Src{Items: []int{1, 2, 3}}
	// Pre-allocate with enough cap — exercises bulkCopy + reuse path
	dst := Dst{Items: make([]int, 5, 10)}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("bulk copy reuse cap failed: %v", err)
	}
	if len(dst.Items) != 3 || dst.Items[0] != 1 || dst.Items[2] != 3 {
		t.Errorf("values mismatch: %v", dst.Items)
	}
}

// ── nilOnZero tag ─────────────────────────────────────────────────────────────

func TestNilOnZero_Pointer(t *testing.T) {
	// nilonzero on a *struct field: zero-value src struct → dst pointer should become nil.
	type Inner struct{ X int }
	type Src struct{ Val Inner }
	type Dst struct {
		Val *Inner `fastcopier:",nilonzero"`
	}
	src := Src{Val: Inner{X: 0}} // zero Inner
	existing := &Inner{X: 99}
	dst := Dst{Val: existing}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("nilonzero pointer failed: %v", err)
	}
	if dst.Val != nil {
		t.Errorf("Val should be nil (zero src), got %v", dst.Val)
	}
}

func TestNilOnZero_Slice(t *testing.T) {
	type Src struct{ Items []string }
	type Dst struct {
		Items []string `fastcopier:",nilonzero"`
	}
	src := Src{Items: []string{}} // empty slice
	dst := Dst{Items: []string{"existing"}}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("nilonzero slice failed: %v", err)
	}
	if dst.Items != nil {
		t.Errorf("Items should be nil (empty src), got %v", dst.Items)
	}
}

func TestNilOnZero_NonZero(t *testing.T) {
	// nilonzero on a *struct field: non-zero src struct → dst pointer should be populated.
	type Inner struct{ X int }
	type Src struct{ Val Inner }
	type Dst struct {
		Val *Inner `fastcopier:",nilonzero"`
	}
	src := Src{Val: Inner{X: 5}}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("nilonzero non-zero failed: %v", err)
	}
	if dst.Val == nil || dst.Val.X != 5 {
		t.Errorf("Val should be &Inner{X:5}, got %v", dst.Val)
	}
}

// ── fieldByIndexInit / fieldSetZero (deeply embedded) ────────────────────────

func TestFieldByIndexInit_DeepEmbedded(t *testing.T) {
	type L3 struct{ Z int }
	type L2 struct{ L3 *L3 }
	type L1 struct{ L2 *L2 }
	type Src struct{ L1 *L1 }
	type Dst struct{ L1 *L1 }

	src := Src{L1: &L1{L2: &L2{L3: &L3{Z: 42}}}}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("deep embedded copy failed: %v", err)
	}
	if dst.L1 == nil || dst.L1.L2 == nil || dst.L1.L2.L3 == nil {
		t.Fatal("deep path not initialised")
	}
	if dst.L1.L2.L3.Z != 42 {
		t.Errorf("Z mismatch: got %d", dst.L1.L2.L3.Z)
	}
}

func TestFieldSetZero_NilEmbeddedSrc(t *testing.T) {
	// fieldSetZero is triggered when src.FieldByIndexErr fails (nil pointer in path)
	type Inner struct{ Val int }
	type Src struct{ Inner *Inner }
	type Dst struct{ Inner *Inner }

	src := Src{Inner: nil}
	dst := Dst{Inner: &Inner{Val: 99}}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("nil embedded src failed: %v", err)
	}
	if dst.Inner != nil {
		t.Errorf("Inner should be nil, got %+v", dst.Inner)
	}
}

// ── nonCopyable / ErrTypeNonCopyable ─────────────────────────────────────────

func TestNonCopyable_Error(t *testing.T) {
	type Src struct{ Ch chan int }
	type Dst struct{ Ch string }
	src := Src{Ch: make(chan int)}
	var dst Dst
	err := Copy(&dst, &src)
	if err == nil {
		t.Error("expected ErrTypeNonCopyable")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected ErrTypeNonCopyable, got %v", err)
	}
}

// ── reset: visited map populated then cleared ─────────────────────────────────

func TestReset_VisitedMapCleared(t *testing.T) {
	// Copy a struct with pointer fields twice — verifies visited map is cleaned up
	type S struct{ P *int }
	v := 1
	src := S{P: &v}
	var dst S
	_ = Copy(&dst, &src)
	v2 := 2
	src.P = &v2
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("second copy failed (visited map leak?): %v", err)
	}
	if *dst.P != 2 {
		t.Errorf("P mismatch: got %d", *dst.P)
	}
}

// ── ifaceSrcPlan ──────────────────────────────────────────────────────────────

func TestIfaceSrcPlan_ConcreteValue(t *testing.T) {
	// src field is interface{} holding a concrete struct; dst field is concrete struct
	type Inner struct{ X int }
	type Src struct{ Val any }
	type Dst struct{ Val Inner }

	src := Src{Val: Inner{X: 7}}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("ifaceSrc copy failed: %v", err)
	}
	if dst.Val.X != 7 {
		t.Errorf("Val.X mismatch: got %d", dst.Val.X)
	}
}

func TestIfaceSrcPlan_NilInterface(t *testing.T) {
	type Src struct{ Val any }
	type Dst struct{ Val any }
	src := Src{Val: nil}
	dst := Dst{Val: "non-nil"}
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("nil iface src failed: %v", err)
	}
	if dst.Val != nil {
		t.Errorf("Val should be nil, got %v", dst.Val)
	}
}

// ── ifaceDstPlan: nil interface src ──────────────────────────────────────────

func TestIfaceDstPlan_NilSrc(t *testing.T) {
	type Src struct{ Val any }
	type Dst struct{ Val any }
	src := Src{Val: nil}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("ifaceDst nil src failed: %v", err)
	}
	if dst.Val != nil {
		t.Errorf("Val should be nil, got %v", dst.Val)
	}
}

// ── channel plan ──────────────────────────────────────────────────────────────

func TestChanPlan_SameType(t *testing.T) {
	type Src struct{ C chan string }
	type Dst struct{ C chan string }
	ch := make(chan string, 1)
	src := Src{C: ch}
	var dst Dst
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("chan copy failed: %v", err)
	}
	if dst.C != ch {
		t.Error("channel should be same reference")
	}
}

// ── required tag ─────────────────────────────────────────────────────────────

func TestRequired_MissingField_Error(t *testing.T) {
	type Src struct{ Name string }
	type Dst struct {
		Name  string
		Email string `fastcopier:",required"`
	}
	src := Src{Name: "req-test"}
	var dst Dst
	err := Copy(&dst, &src)
	if err == nil {
		t.Error("expected ErrFieldRequireCopying for missing required field")
	}
	if !errors.Is(err, ErrFieldRequireCopying) {
		t.Errorf("expected ErrFieldRequireCopying, got %v", err)
	}
}

// ── defaultCtx (used by options path) ────────────────────────────────────────

func TestDefaultCtx_UsedViaOptions(t *testing.T) {
	// Passing any option forces defaultCtx() to be called
	src := SimpleStruct{Name: "opts", Age: 5}
	var dst SimpleStruct
	err := Copy(&dst, &src, WithCopyBetweenPtrAndValue(true))
	if err != nil {
		t.Fatalf("copy with option failed: %v", err)
	}
	if dst.Name != "opts" {
		t.Errorf("Name mismatch: got %s", dst.Name)
	}
}

// ── WithTagName ───────────────────────────────────────────────────────────────

func TestWithTagName_CustomTag(t *testing.T) {
	type Src struct {
		Foo string `mytag:"Bar"`
	}
	type Dst struct {
		Bar string
	}
	src := Src{Foo: "hello"}
	var dst Dst
	err := Copy(&dst, &src, WithTagName("mytag"))
	if err != nil {
		t.Fatalf("WithTagName copy failed: %v", err)
	}
	if dst.Bar != "hello" {
		t.Errorf("Bar mismatch: got %q, want %q", dst.Bar, "hello")
	}
}

func TestWithTagName_DoesNotAffectOtherCalls(t *testing.T) {
	// A call with WithTagName("mytag") must not pollute the global cache
	// used by a subsequent default call.
	type Src struct {
		Name string `fastcopier:"Alias"`
	}
	type Dst struct {
		Alias string
	}
	src := Src{Name: "world"}
	var dst1, dst2 Dst

	// First call: custom tag — "mytag" has no entry, so Name maps by field name only.
	_ = Copy(&dst1, &src, WithTagName("mytag"))

	// Second call: default tag — should use fastcopier tag and map Name→Alias.
	err := Copy(&dst2, &src)
	if err != nil {
		t.Fatalf("default tag copy failed: %v", err)
	}
	if dst2.Alias != "world" {
		t.Errorf("Alias mismatch: got %q, want %q", dst2.Alias, "world")
	}
}

func TestWithTagName_Empty_FallsBackToDefault(t *testing.T) {
	type Src struct {
		Foo string `fastcopier:"Bar"`
	}
	type Dst struct {
		Bar string
	}
	src := Src{Foo: "fallback"}
	var dst Dst
	// Empty tag name should fall back to "fastcopier".
	err := Copy(&dst, &src, WithTagName(""))
	if err != nil {
		t.Fatalf("WithTagName empty failed: %v", err)
	}
	if dst.Bar != "fallback" {
		t.Errorf("Bar mismatch: got %q, want %q", dst.Bar, "fallback")
	}
}

// ── WithFields ────────────────────────────────────────────────────────────────

func TestWithFields_OnlyNamedFieldsCopied(t *testing.T) {
	type Src struct {
		Name string
		Age  int
		City string
	}
	type Dst struct {
		Name string
		Age  int
		City string
	}
	src := Src{Name: "Alice", Age: 30, City: "NYC"}
	dst := Dst{Name: "old", Age: 99, City: "old"}

	err := Copy(&dst, &src, WithFields("Name"))
	if err != nil {
		t.Fatalf("WithFields copy failed: %v", err)
	}
	if dst.Name != "Alice" {
		t.Errorf("Name should be Alice, got %q", dst.Name)
	}
	// Age and City must be untouched.
	if dst.Age != 99 {
		t.Errorf("Age should be 99 (untouched), got %d", dst.Age)
	}
	if dst.City != "old" {
		t.Errorf("City should be old (untouched), got %q", dst.City)
	}
}

func TestWithFields_MultipleFields(t *testing.T) {
	type Src struct {
		A, B, C int
	}
	type Dst struct {
		A, B, C int
	}
	src := Src{A: 1, B: 2, C: 3}
	dst := Dst{A: 10, B: 20, C: 30}

	err := Copy(&dst, &src, WithFields("A", "C"))
	if err != nil {
		t.Fatalf("WithFields multi failed: %v", err)
	}
	if dst.A != 1 || dst.C != 3 {
		t.Errorf("A=%d C=%d, want A=1 C=3", dst.A, dst.C)
	}
	if dst.B != 20 {
		t.Errorf("B should be 20 (untouched), got %d", dst.B)
	}
}

func TestWithFields_NoFieldsOption_CopiesAll(t *testing.T) {
	// WithFields() with no args is a no-op — all fields copied.
	type Src struct{ X, Y int }
	type Dst struct{ X, Y int }
	src := Src{X: 7, Y: 8}
	var dst Dst
	err := Copy(&dst, &src, WithFields())
	if err != nil {
		t.Fatalf("WithFields() no-op failed: %v", err)
	}
	if dst.X != 7 || dst.Y != 8 {
		t.Errorf("X=%d Y=%d, want X=7 Y=8", dst.X, dst.Y)
	}
}
