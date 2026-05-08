// Code-generator benchmarks
//
// This file benchmarks the functions produced by fastcopier-gen and compares
// them with the reflection-based engine and the manual (hand-written) baseline.
//
// HOW TO READ THE RESULTS
// ──────────────────────
//   There are three tiers for each struct shape:
//
//   BenchmarkManual_*            – hand-written field assignments (absolute ceiling)
//   BenchmarkGenerated_Via_Copy_* – fastcopier.Copy() routed to generated code via
//                                   the customRegistry hook registered in init().
//                                   Shows overhead of the sync.Map dispatch.
//   BenchmarkGeneratedDirect_*   – direct call to CopyXToX (zero dispatch overhead,
//                                   closest to manual).
//
//   To compare the pure reflection path (no generated code), run:
//     go test -bench=. -tags fastcopier_no_gen -benchmem -benchtime=3s
//
//   The existing BenchmarkFastCopier_* benchmarks in benchmark_test.go will also
//   route to the generated code when this file is compiled (same init() hook), so
//   they represent the "drop-in" upgrade a user gets for free after running
//   go generate.
//
//go:build !fastcopier_no_gen

package benchmarks

import (
	"testing"

	fastcopier "github.com/expego/fastcopier"
)

// ── Simple Struct ─────────────────────────────────────────────────────────────

// BenchmarkGenerated_Via_Copy_Simple shows the overhead of going through
// fastcopier.Copy() when the generated function is registered.
// Expected: slightly above BenchmarkGeneratedDirect_Simple (1 sync.Map lookup).
func BenchmarkGenerated_Via_Copy_Simple(b *testing.B) {
	src := BenchSimple{Name: "John Doe", Age: 30, Email: "john@example.com", Score: 95.5, Active: true}
	var dst BenchSimple
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fastcopier.Copy(&dst, &src)
	}
}

// BenchmarkGeneratedDirect_Simple calls the generated function directly — zero
// dispatch overhead, closest to the manual baseline.
func BenchmarkGeneratedDirect_Simple(b *testing.B) {
	src := BenchSimple{Name: "John Doe", Age: 30, Email: "john@example.com", Score: 95.5, Active: true}
	var dst BenchSimple
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CopyBenchSimpleToBenchSimple(&dst, &src)
	}
}

// ── Nested Struct ─────────────────────────────────────────────────────────────

// BenchmarkGenerated_Via_Copy_Nested shows fastcopier.Copy() dispatch to the
// generated CopyBenchNestedToBenchNested.
func BenchmarkGenerated_Via_Copy_Nested(b *testing.B) {
	src := BenchNested{
		ID:      1,
		Profile: BenchSimple{Name: "Jane Doe", Age: 25, Email: "jane@example.com", Score: 88.0, Active: true},
		Tags:    []string{"golang", "performance", "testing"},
		Scores:  []int{1, 2, 3, 4, 5},
	}
	var dst BenchNested
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fastcopier.Copy(&dst, &src)
	}
}

// BenchmarkGeneratedDirect_Nested calls CopyBenchNestedToBenchNested directly.
func BenchmarkGeneratedDirect_Nested(b *testing.B) {
	src := BenchNested{
		ID:      1,
		Profile: BenchSimple{Name: "Jane Doe", Age: 25, Email: "jane@example.com", Score: 88.0, Active: true},
		Tags:    []string{"golang", "performance", "testing"},
		Scores:  []int{1, 2, 3, 4, 5},
	}
	var dst BenchNested
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CopyBenchNestedToBenchNested(&dst, &src)
	}
}

// ── Complex Struct ────────────────────────────────────────────────────────────

// BenchmarkGenerated_Via_Copy_Complex shows fastcopier.Copy() dispatch to the
// generated CopyBenchComplexToBenchComplex.
func BenchmarkGenerated_Via_Copy_Complex(b *testing.B) {
	src := BenchComplex{
		ID:   100,
		Name: "Complex Test",
		Nested: BenchNested{
			ID:      1,
			Profile: BenchSimple{Name: "Nested Profile", Age: 35, Email: "nested@example.com"},
			Tags:    []string{"tag1", "tag2"},
			Scores:  []int{10, 20, 30},
		},
		Items:    []BenchSimple{{Name: "Item1", Age: 10}, {Name: "Item2", Age: 20}, {Name: "Item3", Age: 30}},
		Metadata: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
	}
	var dst BenchComplex
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fastcopier.Copy(&dst, &src)
	}
}

// BenchmarkGeneratedDirect_Complex calls CopyBenchComplexToBenchComplex directly.
// Items is a []BenchSimple (flat struct slice) so the generated code uses
// builtin copy() — much cheaper than element-by-element reflection.
func BenchmarkGeneratedDirect_Complex(b *testing.B) {
	src := BenchComplex{
		ID:   100,
		Name: "Complex Test",
		Nested: BenchNested{
			ID:      1,
			Profile: BenchSimple{Name: "Nested Profile", Age: 35, Email: "nested@example.com"},
			Tags:    []string{"tag1", "tag2"},
			Scores:  []int{10, 20, 30},
		},
		Items:    []BenchSimple{{Name: "Item1", Age: 10}, {Name: "Item2", Age: 20}, {Name: "Item3", Age: 30}},
		Metadata: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
	}
	var dst BenchComplex
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CopyBenchComplexToBenchComplex(&dst, &src)
	}
}
