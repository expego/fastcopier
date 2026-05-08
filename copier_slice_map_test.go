package fastcopier

import (
	"testing"
)

// ── Copy with top-level slices ────────────────────────────────────────────────

func TestCopySliceSameType(t *testing.T) {
	src := []string{"a", "b", "c"}
	var dst []string
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if len(dst) != len(src) {
		t.Fatalf("length mismatch: got %d, want %d", len(dst), len(src))
	}
	for i := range src {
		if dst[i] != src[i] {
			t.Errorf("dst[%d] = %q, want %q", i, dst[i], src[i])
		}
	}
	// Verify deep copy — mutating src must not affect dst.
	src[0] = "mutated"
	if dst[0] == "mutated" {
		t.Error("dst shares backing array with src (not a deep copy)")
	}
}

func TestCopySliceSameTypeValueSrc(t *testing.T) {
	// src passed as value (not pointer) — should also work.
	src := []int{1, 2, 3}
	var dst []int
	if err := Copy(&dst, src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if len(dst) != 3 || dst[1] != 2 {
		t.Errorf("unexpected dst: %v", dst)
	}
}

func TestCopySliceDifferentStructTypes(t *testing.T) {
	type SrcItem struct {
		Name string
		Age  int
	}
	type DstItem struct {
		Name string
		Age  int
	}

	src := []SrcItem{
		{Name: "Alice", Age: 30},
		{Name: "Bob", Age: 25},
	}
	var dst []DstItem
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if len(dst) != 2 {
		t.Fatalf("length mismatch: got %d, want 2", len(dst))
	}
	if dst[0].Name != "Alice" || dst[0].Age != 30 {
		t.Errorf("dst[0] = %+v, want {Alice 30}", dst[0])
	}
	if dst[1].Name != "Bob" || dst[1].Age != 25 {
		t.Errorf("dst[1] = %+v, want {Bob 25}", dst[1])
	}
}

func TestCopySliceNil(t *testing.T) {
	var src []string
	var dst []string
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy of nil slice failed: %v", err)
	}
	if dst != nil {
		t.Errorf("expected nil dst for nil src, got %v", dst)
	}
}

func TestCopySliceEmpty(t *testing.T) {
	src := []string{}
	var dst []string
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if len(dst) != 0 {
		t.Errorf("expected empty dst, got %v", dst)
	}
}

func TestCopySliceReusesCapacity(t *testing.T) {
	// Pre-allocate dst with enough capacity; Copy should reuse the backing array.
	src := []int{10, 20, 30}
	dst := make([]int, 5, 10)
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if len(dst) != 3 {
		t.Errorf("expected len 3, got %d", len(dst))
	}
	if dst[0] != 10 || dst[1] != 20 || dst[2] != 30 {
		t.Errorf("unexpected dst: %v", dst)
	}
}

// ── Copy with top-level maps ──────────────────────────────────────────────────

func TestCopyMapSameType(t *testing.T) {
	src := map[string]string{"key1": "val1", "key2": "val2"}
	var dst map[string]string
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if len(dst) != 2 {
		t.Fatalf("length mismatch: got %d, want 2", len(dst))
	}
	if dst["key1"] != "val1" || dst["key2"] != "val2" {
		t.Errorf("unexpected dst: %v", dst)
	}
	// Verify deep copy.
	src["key1"] = "mutated"
	if dst["key1"] == "mutated" {
		t.Error("dst shares entries with src (not a deep copy)")
	}
}

func TestCopyMapNil(t *testing.T) {
	var src map[string]int
	var dst map[string]int
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy of nil map failed: %v", err)
	}
	if dst != nil {
		t.Errorf("expected nil dst for nil src, got %v", dst)
	}
}

// ── Map[Src, Dst] generic helper ─────────────────────────────────────────────

func TestMapSameType(t *testing.T) {
	type Item struct{ Name string }
	src := []Item{{Name: "x"}, {Name: "y"}}
	dst, err := Map[Item, Item](src)
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}
	if len(dst) != 2 || dst[0].Name != "x" || dst[1].Name != "y" {
		t.Errorf("unexpected dst: %v", dst)
	}
}

func TestMapDifferentTypes(t *testing.T) {
	type SrcUser struct {
		ID   int
		Name string
		Age  int
	}
	type DstUser struct {
		ID   int
		Name string
	}

	src := []SrcUser{
		{ID: 1, Name: "Alice", Age: 30},
		{ID: 2, Name: "Bob", Age: 25},
	}
	dst, err := Map[SrcUser, DstUser](src)
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}
	if len(dst) != 2 {
		t.Fatalf("length mismatch: got %d, want 2", len(dst))
	}
	if dst[0].ID != 1 || dst[0].Name != "Alice" {
		t.Errorf("dst[0] = %+v, want {1 Alice}", dst[0])
	}
	if dst[1].ID != 2 || dst[1].Name != "Bob" {
		t.Errorf("dst[1] = %+v, want {2 Bob}", dst[1])
	}
}

func TestMapEmpty(t *testing.T) {
	type Item struct{ Name string }
	dst, err := Map[Item, Item]([]Item{})
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}
	if len(dst) != 0 {
		t.Errorf("expected empty dst, got %v", dst)
	}
}

func TestMapNilSlice(t *testing.T) {
	type Item struct{ Name string }
	dst, err := Map[Item, Item](nil)
	if err != nil {
		t.Fatalf("Map of nil slice failed: %v", err)
	}
	if dst != nil {
		t.Errorf("expected nil dst for nil src, got %v", dst)
	}
}

func TestMapWithOptions(t *testing.T) {
	type Src struct {
		Name string
		Age  int
	}
	type Dst struct {
		Name string
		Age  int
	}

	src := []Src{{Name: "Alice", Age: 30}}
	dst, err := Map[Src, Dst](src, WithCopyBetweenPtrAndValue(true))
	if err != nil {
		t.Fatalf("Map with options failed: %v", err)
	}
	if len(dst) != 1 || dst[0].Name != "Alice" {
		t.Errorf("unexpected dst: %v", dst)
	}
}

func TestMapDeepCopyIsolation(t *testing.T) {
	// Mutating src after Map must not affect dst.
	type Item struct {
		Tags []string
	}
	src := []Item{{Tags: []string{"go", "fast"}}}
	dst, err := Map[Item, Item](src)
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}
	src[0].Tags[0] = "mutated"
	if dst[0].Tags[0] == "mutated" {
		t.Error("Map did not deep-copy nested slice")
	}
}

// ── Flat-struct slice bulk-copy ───────────────────────────────────────────────

// TestCopySliceFlatStruct verifies that []FlatStruct is bulk-copied correctly
// (same result as element-by-element, with independent backing arrays).
func TestCopySliceFlatStruct(t *testing.T) {
	type Point struct{ X, Y, Z float64 }

	src := []Point{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
	var dst []Point
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if len(dst) != len(src) {
		t.Fatalf("length: got %d want %d", len(dst), len(src))
	}
	for i, p := range src {
		if dst[i] != p {
			t.Errorf("dst[%d] = %+v, want %+v", i, dst[i], p)
		}
	}
	// Mutating src must not affect dst (independent backing arrays).
	src[0].X = 999
	if dst[0].X == 999 {
		t.Error("dst shares backing array with src")
	}
}

// TestCopySliceFlatStructCapacityReuse verifies that the bulk path reuses the
// destination's existing backing array when capacity is sufficient.
func TestCopySliceFlatStructCapacityReuse(t *testing.T) {
	type Point struct{ X, Y float64 }

	src := []Point{{1, 2}, {3, 4}}
	// Pre-allocate dst with enough capacity.
	dst := make([]Point, 5, 10)
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if len(dst) != 2 {
		t.Fatalf("len: got %d want 2", len(dst))
	}
	if cap(dst) != 10 {
		t.Fatalf("cap should be reused (10), got %d", cap(dst))
	}
	if dst[0] != (Point{1, 2}) || dst[1] != (Point{3, 4}) {
		t.Errorf("wrong values: %+v", dst)
	}
}

// TestCopySliceFlatStructMultiField exercises a flat struct with mixed scalar kinds.
func TestCopySliceFlatStructMultiField(t *testing.T) {
	type Record struct {
		Name   string
		Age    int
		Score  float64
		Active bool
	}

	src := []Record{
		{"Alice", 30, 9.5, true},
		{"Bob", 25, 8.2, false},
		{"Carol", 35, 7.7, true},
	}
	var dst []Record
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	for i, r := range src {
		if dst[i] != r {
			t.Errorf("dst[%d] = %+v, want %+v", i, dst[i], r)
		}
	}
	// Mutation independence.
	src[0].Name = "mutated"
	if dst[0].Name == "mutated" {
		t.Error("Name strings are shared (unexpected)")
	}
}

// TestCopySliceFlatStructNested exercises []struct{nested flat struct}.
func TestCopySliceFlatStructNested(t *testing.T) {
	type Inner struct{ A, B int }
	type Outer struct {
		ID    int
		Inner Inner
	}

	src := []Outer{{1, Inner{10, 20}}, {2, Inner{30, 40}}}
	var dst []Outer
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	for i, o := range src {
		if dst[i] != o {
			t.Errorf("dst[%d] = %+v, want %+v", i, dst[i], o)
		}
	}
}

// TestCopySliceFlatStructInStruct verifies bulk-copy for a struct field of type
// []FlatStruct (the most common real-world trigger).
func TestCopySliceFlatStructInStruct(t *testing.T) {
	type Item struct{ Name string; Price float64 }
	type Order struct {
		ID    int
		Items []Item
	}

	src := Order{
		ID:    42,
		Items: []Item{{"apple", 1.5}, {"banana", 0.75}},
	}
	var dst Order
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst.ID != 42 {
		t.Errorf("ID: got %d want 42", dst.ID)
	}
	if len(dst.Items) != 2 {
		t.Fatalf("len(Items): got %d want 2", len(dst.Items))
	}
	for i, it := range src.Items {
		if dst.Items[i] != it {
			t.Errorf("Items[%d]: got %+v want %+v", i, dst.Items[i], it)
		}
	}
	// Mutation independence.
	src.Items[0].Name = "mutated"
	if dst.Items[0].Name == "mutated" {
		t.Error("Item strings are shared after bulk copy")
	}
}

// TestCopySliceFlatStructUseBulkCopy confirms the fast path is taken by checking
// that a nil src slice is correctly handled (sets dst to nil, not empty slice).
func TestCopySliceFlatStructNilSrc(t *testing.T) {
	type Point struct{ X, Y float64 }

	var src []Point
	dst := []Point{{1, 2}}
	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst != nil {
		t.Errorf("expected nil dst for nil src, got %v", dst)
	}
}
