package gentest_test

// Tests for the generated copy functions in copy_gen.go and copy_user_gen.go.
// Exercises direct calls, all nil/non-nil slice/map branches, and verifies that
// fastcopier.Copy routes through the registered generated functions.

import (
	"testing"

	fastcopier "github.com/expego/fastcopier"

	"github.com/expego/fastcopier/internal/gentest"
)

// ── CopySimpleToSimple ────────────────────────────────────────────────────────

func TestCopySimpleToSimple_DirectCall(t *testing.T) {
	src := gentest.Simple{
		Name:   "Alice",
		Age:    30,
		Email:  "alice@example.com",
		Score:  9.5,
		Active: true,
	}
	var dst gentest.Simple
	if err := gentest.CopySimpleToSimple(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst != src {
		t.Errorf("dst = %+v, want %+v", dst, src)
	}
}

func TestCopySimpleToSimple_ViaFastcopier(t *testing.T) {
	// RegisterCopier routes fastcopier.Copy to the generated function.
	src := gentest.Simple{Name: "Bob", Age: 25, Email: "bob@example.com", Score: 7.0, Active: false}
	var dst gentest.Simple
	if err := fastcopier.Copy(&dst, &src); err != nil {
		t.Fatalf("fastcopier.Copy error: %v", err)
	}
	if dst != src {
		t.Errorf("dst = %+v, want %+v", dst, src)
	}
}

// ── CopyNestedToNested ────────────────────────────────────────────────────────

func TestCopyNestedToNested_NilSlices(t *testing.T) {
	src := gentest.Nested{ID: 1, Tags: nil, Scores: nil}
	dst := gentest.Nested{Tags: []string{"old"}, Scores: []int{99}}
	if err := gentest.CopyNestedToNested(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Tags != nil {
		t.Errorf("Tags should be nil, got %v", dst.Tags)
	}
	if dst.Scores != nil {
		t.Errorf("Scores should be nil, got %v", dst.Scores)
	}
}

func TestCopyNestedToNested_ReuseCapacity(t *testing.T) {
	// Pre-allocate dst slices with enough capacity → no new allocation expected.
	src := gentest.Nested{
		ID:     2,
		Tags:   []string{"go", "fastcopier"},
		Scores: []int{10, 20, 30},
	}
	dst := gentest.Nested{
		Tags:   make([]string, 0, 10), // capacity > len(src.Tags)
		Scores: make([]int, 0, 10),
	}
	if err := gentest.CopyNestedToNested(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dst.Tags) != len(src.Tags) || dst.Tags[0] != "go" {
		t.Errorf("Tags = %v, want %v", dst.Tags, src.Tags)
	}
	if len(dst.Scores) != len(src.Scores) || dst.Scores[2] != 30 {
		t.Errorf("Scores = %v, want %v", dst.Scores, src.Scores)
	}
}

func TestCopyNestedToNested_AllocateNewSlice(t *testing.T) {
	// dst slices have insufficient capacity → must allocate new backing arrays.
	src := gentest.Nested{
		ID:     3,
		Tags:   []string{"a", "b", "c", "d", "e"},
		Scores: []int{1, 2, 3, 4, 5},
	}
	dst := gentest.Nested{
		Tags:   make([]string, 0, 2), // too small
		Scores: make([]int, 0, 1),
	}
	if err := gentest.CopyNestedToNested(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dst.Tags) != 5 || dst.Tags[4] != "e" {
		t.Errorf("Tags = %v, want %v", dst.Tags, src.Tags)
	}
	if len(dst.Scores) != 5 || dst.Scores[4] != 5 {
		t.Errorf("Scores = %v, want %v", dst.Scores, src.Scores)
	}
}

func TestCopyNestedToNested_ProfileCopied(t *testing.T) {
	src := gentest.Nested{
		ID:      10,
		Profile: gentest.Simple{Name: "nested-profile", Age: 42},
	}
	var dst gentest.Nested
	if err := gentest.CopyNestedToNested(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.ID != 10 || dst.Profile.Name != "nested-profile" || dst.Profile.Age != 42 {
		t.Errorf("dst = %+v, want matching src", dst)
	}
}

// ── CopyComplexToComplex ──────────────────────────────────────────────────────

func TestCopyComplexToComplex_NilSliceAndMap(t *testing.T) {
	src := gentest.Complex{ID: 5, Name: "nil-test", Items: nil, Metadata: nil}
	dst := gentest.Complex{
		Items:    []gentest.Simple{{Name: "old"}},
		Metadata: map[string]string{"k": "v"},
	}
	if err := gentest.CopyComplexToComplex(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Items != nil {
		t.Errorf("Items should be nil, got %v", dst.Items)
	}
	if dst.Metadata != nil {
		t.Errorf("Metadata should be nil, got %v", dst.Metadata)
	}
}

func TestCopyComplexToComplex_NonNilSliceAndMap(t *testing.T) {
	src := gentest.Complex{
		ID:   7,
		Name: "full",
		Nested: gentest.Nested{
			ID:     7,
			Tags:   []string{"x"},
			Scores: []int{1},
		},
		Items: []gentest.Simple{
			{Name: "item1", Age: 1},
			{Name: "item2", Age: 2},
		},
		Metadata: map[string]string{"env": "prod", "ver": "1"},
	}
	var dst gentest.Complex
	if err := gentest.CopyComplexToComplex(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.ID != 7 || dst.Name != "full" {
		t.Errorf("scalar fields mismatch: %+v", dst)
	}
	if len(dst.Items) != 2 || dst.Items[1].Name != "item2" {
		t.Errorf("Items = %v", dst.Items)
	}
	if dst.Metadata["env"] != "prod" || dst.Metadata["ver"] != "1" {
		t.Errorf("Metadata = %v", dst.Metadata)
	}
	if dst.Nested.ID != 7 || len(dst.Nested.Tags) != 1 {
		t.Errorf("Nested = %+v", dst.Nested)
	}
}

func TestCopyComplexToComplex_ItemsReuseCapacity(t *testing.T) {
	src := gentest.Complex{
		Items: []gentest.Simple{{Name: "a"}, {Name: "b"}},
	}
	dst := gentest.Complex{
		Items: make([]gentest.Simple, 0, 10), // large cap → reuse
	}
	if err := gentest.CopyComplexToComplex(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dst.Items) != 2 || dst.Items[0].Name != "a" {
		t.Errorf("Items = %v", dst.Items)
	}
}

// ── CopyUserEntityToUserDTO ───────────────────────────────────────────────────

func TestCopyUserEntityToUserDTO_DirectCall(t *testing.T) {
	src := gentest.UserEntity{
		ID:       99,
		Name:     "Carol",
		Email:    "carol@example.com",
		Password: "secret", // must NOT appear in dst
	}
	var dst gentest.UserDTO
	if err := gentest.CopyUserEntityToUserDTO(&dst, &src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.ID != 99 || dst.Name != "Carol" || dst.Email != "carol@example.com" {
		t.Errorf("dst = %+v", dst)
	}
}

func TestCopyUserEntityToUserDTO_ViaFastcopier(t *testing.T) {
	src := gentest.UserEntity{ID: 1, Name: "Dave", Email: "dave@example.com", Password: "pw"}
	var dst gentest.UserDTO
	if err := fastcopier.Copy(&dst, &src); err != nil {
		t.Fatalf("fastcopier.Copy error: %v", err)
	}
	if dst.ID != 1 || dst.Name != "Dave" || dst.Email != "dave@example.com" {
		t.Errorf("dst = %+v", dst)
	}
}
