package fastcopier_test

// Tests targeting inspect.go branches not reached by existing tests:
//   - inspectPlan: *derefPlan and *addrPlan top-level cases
//   - classifyPlan: skipPlan case
//   - fieldNameByIndex: recursive case through anonymous embedded *T pointer

import (
	"testing"

	"github.com/expego/fastcopier"
)

// ── inspectPlan: *derefPlan top-level ────────────────────────────────────────
// To reach the *derefPlan case, Inspect must receive a **T src so that after the
// outer-pointer deref srcType = *T, which causes resolvePlan to return a derefPlan.

func TestInspect_TopLevel_DerefPlan(t *testing.T) {
	type S struct{ X int }

	src := &S{X: 1} // type *S
	// &src is **S; after Inspect's one-level deref srcType = *S → derefPlan
	plan, err := fastcopier.Inspect(&S{}, &src)
	if err != nil {
		t.Fatalf("Inspect error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil InspectPlan")
	}
	// The derefPlan recurses into a structPlan for S→S, so Fields must be populated.
	if len(plan.Fields) == 0 {
		t.Error("expected at least one field mapping from structPlan inside derefPlan")
	}
}

// ── inspectPlan: *addrPlan top-level ─────────────────────────────────────────
// To reach the *addrPlan case, Inspect must receive a **T dst so that after the
// outer-pointer deref dstType = *T (pointer), which causes resolvePlan to return
// an addrPlan for the (dstType=*T, srcType=S) pair.

func TestInspect_TopLevel_AddrPlan(t *testing.T) {
	type S struct{ X int }

	dstPtr := (*S)(nil) // type *S, nil
	// &dstPtr is **S; after Inspect's deref dstType = *S → addrPlan
	plan, err := fastcopier.Inspect(&dstPtr, S{X: 1})
	if err != nil {
		t.Fatalf("Inspect error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil InspectPlan")
	}
	// addrPlan wraps a structPlan for S→S, so Fields must be populated.
	if len(plan.Fields) == 0 {
		t.Error("expected at least one field mapping from structPlan inside addrPlan")
	}
}

// ── classifyPlan: skipPlan case ───────────────────────────────────────────────
// When WithIgnoreNonCopyableTypes(true) is set and a field has incompatible types,
// resolvePlan returns skipPlan and classifyPlan should report Action="skip".

func TestInspect_ClassifyPlan_Skip(t *testing.T) {
	type Src struct {
		Name string
		Fn   func() // scalar kind (Func), incompatible with chan int below
	}
	type Dst struct {
		Name string
		Fn   chan int // non-copyable mismatch
	}

	plan, err := fastcopier.Inspect(
		&Dst{}, &Src{},
		fastcopier.WithIgnoreNonCopyableTypes(true),
	)
	if err != nil {
		t.Fatalf("Inspect error: %v", err)
	}

	fn := findField(plan.Fields, "Fn")
	if fn == nil {
		t.Fatal("expected field 'Fn' in plan")
	}
	if fn.Action != "skip" {
		t.Errorf("Fn action = %q, want %q", fn.Action, "skip")
	}
}

// ── fieldNameByIndex: recursive case through anonymous embedded *T pointer ───
// When a dst struct embeds a *T pointer anonymously, the promoted field has
// dstIndex=[0,X]. fieldNameByIndex must handle the pointer-typed intermediate step.

// ── classifyPlan: nil plan + struct srcType → "deep-copy" ────────────────────
// When fieldPlan.elem is nil (flat struct, same type), classifyPlan checks
// srcType.Kind() == Struct and returns "deep-copy" (not "assign").

func TestInspect_ClassifyPlan_NilElem_FlatStructField(t *testing.T) {
	// Inner is a flat struct (all scalars). Outer is not flat because of the
	// []int slice field, so resolvePlan builds a structPlan for Outer→Outer
	// instead of taking the single-assign fast path. That causes inspectStructPlan
	// to call classifyPlan(nil, Inner-struct, Inner-struct) which returns "deep-copy".
	type Inner struct{ A, B int }
	type Outer struct {
		Inner Inner
		Data  []int // makes Outer non-flat → structPlan is built
	}

	plan, err := fastcopier.Inspect(&Outer{}, &Outer{})
	if err != nil {
		t.Fatalf("Inspect error: %v", err)
	}
	f := findField(plan.Fields, "Inner")
	if f == nil {
		t.Fatal("expected field 'Inner' in plan")
	}
	// A flat struct field copied via a single Set should still report "deep-copy"
	// because conceptually copying a struct is a deep copy.
	if f.Action != "deep-copy" {
		t.Errorf("Inner action = %q, want %q", f.Action, "deep-copy")
	}
}

func TestInspect_FieldNameByIndex_AnonPointerEmbed(t *testing.T) {
	type InnerV struct{ Value int }
	type OuterP struct{ *InnerV } // anonymous embedded pointer → Value at dstIndex=[0,0]
	type Src struct{ Value int }

	// Inspect with an initialised OuterP so Inspect gets a valid reflect.Value.
	plan, err := fastcopier.Inspect(
		&OuterP{InnerV: &InnerV{}},
		&Src{Value: 5},
	)
	if err != nil {
		t.Fatalf("Inspect error: %v", err)
	}

	// fieldNameByIndex recurses through *InnerV to resolve "Value".
	vf := findField(plan.Fields, "Value")
	if vf == nil {
		t.Fatal("expected field mapping for 'Value'")
	}
	if vf.DstField != "Value" {
		t.Errorf("DstField = %q, want %q", vf.DstField, "Value")
	}
}
