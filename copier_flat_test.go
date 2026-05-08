package fastcopier

import (
	"reflect"
	"testing"
)

// ── isFlatType unit tests ─────────────────────────────────────────────────────

func TestIsFlatType_Scalars(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
		want bool
	}{
		{"bool", reflect.TypeOf(false), true},
		{"int", reflect.TypeOf(0), true},
		{"int8", reflect.TypeOf(int8(0)), true},
		{"int32", reflect.TypeOf(int32(0)), true},
		{"int64", reflect.TypeOf(int64(0)), true},
		{"uint", reflect.TypeOf(uint(0)), true},
		{"float32", reflect.TypeOf(float32(0)), true},
		{"float64", reflect.TypeOf(float64(0)), true},
		{"string", reflect.TypeOf(""), true},
		{"complex128", reflect.TypeOf(complex128(0)), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFlatType(tc.typ); got != tc.want {
				t.Errorf("isFlatType(%v) = %v, want %v", tc.typ, got, tc.want)
			}
		})
	}
}

func TestIsFlatType_NonFlat(t *testing.T) {
	type withSlice struct{ S []int }
	type withMap struct{ M map[string]int }
	type withPtr struct{ P *int }
	type withIface struct{ I any }
	type withChan struct{ C chan int }

	cases := []struct {
		name string
		typ  reflect.Type
	}{
		{"[]int", reflect.TypeOf([]int{})},
		{"map[string]int", reflect.TypeOf(map[string]int{})},
		{"*int", reflect.TypeOf((*int)(nil))},
		{"struct with slice", reflect.TypeOf(withSlice{})},
		{"struct with map", reflect.TypeOf(withMap{})},
		{"struct with ptr", reflect.TypeOf(withPtr{})},
		{"struct with iface", reflect.TypeOf(withIface{})},
		{"struct with chan", reflect.TypeOf(withChan{})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if isFlatType(tc.typ) {
				t.Errorf("isFlatType(%v) = true, want false", tc.typ)
			}
		})
	}
}

func TestIsFlatType_FlatStructs(t *testing.T) {
	type allScalars struct {
		Name   string
		Age    int
		Score  float64
		Active bool
	}
	type nested struct {
		A allScalars
		B int
	}
	type arrayField struct {
		Coords [3]float64
	}

	cases := []struct {
		name string
		typ  reflect.Type
		want bool
	}{
		{"all scalars", reflect.TypeOf(allScalars{}), true},
		{"nested flat structs", reflect.TypeOf(nested{}), true},
		{"array of float64", reflect.TypeOf([3]float64{}), true},
		{"struct with flat array", reflect.TypeOf(arrayField{}), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFlatType(tc.typ); got != tc.want {
				t.Errorf("isFlatType(%v) = %v, want %v", tc.typ, got, tc.want)
			}
		})
	}
}

// ── Flat struct copy correctness tests ───────────────────────────────────────

// flatSimple mirrors BenchSimple — all scalar fields, same type copy.
type flatSimple struct {
	Name   string
	Age    int
	Email  string
	Score  float64
	Active bool
}

// flatWithArray contains a fixed-size array — also flat.
type flatWithArray struct {
	Label  string
	Coords [3]float64
	Count  int
}

// flatNested is a struct whose fields are themselves flat structs.
type flatNested struct {
	A flatSimple
	B flatSimple
	N int
}

func TestFlatStructCopy_AllFieldsCorrect(t *testing.T) {
	src := flatSimple{Name: "Alice", Age: 28, Email: "alice@example.com", Score: 9.5, Active: true}
	var dst flatSimple

	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst != src {
		t.Errorf("got %+v, want %+v", dst, src)
	}
}

func TestFlatStructCopy_MutationIndependence(t *testing.T) {
	// Mutating src after copy must not change dst (proves deep copy, not alias).
	src := flatSimple{Name: "Bob", Age: 35}
	var dst flatSimple

	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	src.Name = "Charlie"
	src.Age = 99

	if dst.Name != "Bob" {
		t.Errorf("dst.Name changed after mutating src: got %q", dst.Name)
	}
	if dst.Age != 35 {
		t.Errorf("dst.Age changed after mutating src: got %d", dst.Age)
	}
}

func TestFlatStructCopy_WithArray(t *testing.T) {
	src := flatWithArray{Label: "origin", Coords: [3]float64{1.1, 2.2, 3.3}, Count: 7}
	var dst flatWithArray

	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst != src {
		t.Errorf("got %+v, want %+v", dst, src)
	}
}

// TestFlatStructCopy_AsNestedField verifies that a flat struct embedded as a
// field inside a non-flat parent is also copied correctly via the fast path.
func TestFlatStructCopy_AsNestedField(t *testing.T) {
	type parent struct {
		Profile flatSimple
		Tags    []string // makes parent non-flat
	}

	src := parent{
		Profile: flatSimple{Name: "Dave", Age: 42, Score: 8.0, Active: false},
		Tags:    []string{"go", "perf"},
	}
	var dst parent

	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst.Profile != src.Profile {
		t.Errorf("Profile mismatch: got %+v, want %+v", dst.Profile, src.Profile)
	}
	if len(dst.Tags) != len(src.Tags) || dst.Tags[0] != src.Tags[0] {
		t.Errorf("Tags mismatch: got %v, want %v", dst.Tags, src.Tags)
	}
}

func TestFlatStructCopy_DeepFlatNested(t *testing.T) {
	src := flatNested{
		A: flatSimple{Name: "X", Age: 1, Score: 1.1},
		B: flatSimple{Name: "Y", Age: 2, Score: 2.2},
		N: 42,
	}
	var dst flatNested

	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst != src {
		t.Errorf("got %+v, want %+v", dst, src)
	}
}

func TestFlatStructCopy_ZeroValue(t *testing.T) {
	src := flatSimple{} // all zero
	dst := flatSimple{Name: "old", Age: 99, Score: 3.14, Active: true}

	if err := Copy(&dst, &src); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if dst != src {
		t.Errorf("expected zero dst, got %+v", dst)
	}
}
