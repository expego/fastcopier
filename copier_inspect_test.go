package fastcopier_test

import (
	"strings"
	"testing"

	"github.com/expego/fastcopier"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func fieldNames(fields []fastcopier.FieldMapping) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.SrcField
	}
	return names
}

func findField(fields []fastcopier.FieldMapping, srcField string) *fastcopier.FieldMapping {
	for i := range fields {
		if fields[i].SrcField == srcField {
			return &fields[i]
		}
	}
	return nil
}

// ── basic struct ──────────────────────────────────────────────────────────────

func TestInspect_BasicStruct(t *testing.T) {
	type Src struct {
		Name  string
		Age   int
		Email string
	}
	type Dst struct {
		Name  string
		Age   int
		Email string
	}

	plan, err := fastcopier.Inspect(&Dst{}, &Src{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Fields) != 3 {
		t.Errorf("want 3 fields, got %d: %v", len(plan.Fields), fieldNames(plan.Fields))
	}
	if len(plan.Skipped) != 0 {
		t.Errorf("want 0 skipped, got %v", plan.Skipped)
	}
}

// ── partial match — extra src fields ─────────────────────────────────────────

func TestInspect_PartialMatch(t *testing.T) {
	type Src struct {
		Name  string
		Age   int
		Email string
	}
	type Dst struct {
		Name string
		Age  int
	}

	plan, err := fastcopier.Inspect(&Dst{}, &Src{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Fields) != 2 {
		t.Errorf("want 2 fields, got %d", len(plan.Fields))
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0] != "Email" {
		t.Errorf("want [Email] skipped, got %v", plan.Skipped)
	}
}

// ── tag rename ────────────────────────────────────────────────────────────────

func TestInspect_TagRename(t *testing.T) {
	type Src struct {
		UserName string `fastcopier:"Name"`
	}
	type Dst struct {
		Name string
	}

	plan, err := fastcopier.Inspect(&Dst{}, &Src{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Fields) != 1 {
		t.Fatalf("want 1 field, got %d", len(plan.Fields))
	}
	f := plan.Fields[0]
	// SrcField shows the tag-resolved key used for matching ("Name"), not the Go field name.
	// The tag `fastcopier:"Name"` on UserName means the key is "Name".
	if f.SrcField != "Name" && f.SrcField != "UserName" {
		t.Errorf("SrcField: want Name or UserName, got %q", f.SrcField)
	}
	if f.DstField != "Name" {
		t.Errorf("DstField: want Name, got %q", f.DstField)
	}
}

// ── tag skip ──────────────────────────────────────────────────────────────────

func TestInspect_TagSkip(t *testing.T) {
	type Src struct {
		Name     string
		Internal string `fastcopier:"-"`
	}
	type Dst struct {
		Name     string
		Internal string
	}

	plan, err := fastcopier.Inspect(&Dst{}, &Src{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Internal is ignored on src side, so only Name is copied.
	if len(plan.Fields) != 1 {
		t.Errorf("want 1 field, got %d: %v", len(plan.Fields), fieldNames(plan.Fields))
	}
}

// ── type conversion ───────────────────────────────────────────────────────────

func TestInspect_TypeConversion(t *testing.T) {
	type Src struct{ Count int }
	type Dst struct{ Count int64 }

	plan, err := fastcopier.Inspect(&Dst{}, &Src{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Fields) != 1 {
		t.Fatalf("want 1 field, got %d", len(plan.Fields))
	}
	f := plan.Fields[0]
	if f.Action != "convert" {
		t.Errorf("want action=convert, got %q", f.Action)
	}
	if f.SrcType != "int" {
		t.Errorf("SrcType: want int, got %q", f.SrcType)
	}
	if f.DstType != "int64" {
		t.Errorf("DstType: want int64, got %q", f.DstType)
	}
}

// ── nested struct → deep-copy ─────────────────────────────────────────────────

func TestInspect_NestedStruct(t *testing.T) {
	type Address struct{ City string }
	type Src struct{ Addr Address }
	type Dst struct{ Addr Address }

	plan, err := fastcopier.Inspect(&Dst{}, &Src{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Fields) != 1 {
		t.Fatalf("want 1 field, got %d", len(plan.Fields))
	}
	f := plan.Fields[0]
	if f.Action != "deep-copy" {
		t.Errorf("want action=deep-copy, got %q", f.Action)
	}
}

// ── String() output ───────────────────────────────────────────────────────────

func TestInspect_StringOutput(t *testing.T) {
	type Src struct {
		Name  string
		Extra string
	}
	type Dst struct{ Name string }

	plan, err := fastcopier.Inspect(&Dst{}, &Src{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := plan.String()
	if !strings.Contains(s, "Src") && !strings.Contains(s, "fastcopier_test") {
		t.Errorf("String() should contain type names, got:\n%s", s)
	}
	if !strings.Contains(s, "Name") {
		t.Errorf("String() should contain field Name, got:\n%s", s)
	}
	if !strings.Contains(s, "skipped") {
		t.Errorf("String() should mention skipped fields, got:\n%s", s)
	}
}

// ── SrcType / DstType populated ───────────────────────────────────────────────

func TestInspect_TypeNames(t *testing.T) {
	type Src struct{ Name string }
	type Dst struct{ Name string }

	plan, err := fastcopier.Inspect(&Dst{}, &Src{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.SrcType == "" {
		t.Error("SrcType should not be empty")
	}
	if plan.DstType == "" {
		t.Error("DstType should not be empty")
	}
}

// ── error cases ───────────────────────────────────────────────────────────────

func TestInspect_NonPointerDst(t *testing.T) {
	type Src struct{ Name string }
	type Dst struct{ Name string }

	_, err := fastcopier.Inspect(Dst{}, &Src{})
	if err == nil {
		t.Error("expected error for non-pointer dst")
	}
}

func TestInspect_NilDst(t *testing.T) {
	type Src struct{ Name string }

	_, err := fastcopier.Inspect(nil, &Src{})
	if err == nil {
		t.Error("expected error for nil dst")
	}
}

// ── MustRegister ──────────────────────────────────────────────────────────────

func TestMustRegister_Valid(t *testing.T) {
	type Src struct{ Name string }
	type Dst struct{ Name string }

	// Should not panic.
	fastcopier.MustRegister(&Dst{}, &Src{})
}

func TestMustRegister_Panics_NonPointer(t *testing.T) {
	type Src struct{ Name string }
	type Dst struct{ Name string }

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-pointer dst")
		}
	}()
	fastcopier.MustRegister(Dst{}, &Src{})
}
