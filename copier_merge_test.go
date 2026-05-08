package fastcopier

import (
	"errors"
	"testing"
)

// ── Merge ─────────────────────────────────────────────────────────────────────

func TestMerge_BasicPatch(t *testing.T) {
	type User struct {
		Name  string
		Age   int
		Email string
	}

	dst := User{Name: "Alice", Age: 30, Email: "alice@example.com"}
	patch := User{Email: "new@example.com"} // only Email is non-zero

	if err := Merge(&dst, &patch); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if dst.Name != "Alice" {
		t.Errorf("Name should be unchanged: got %q", dst.Name)
	}
	if dst.Age != 30 {
		t.Errorf("Age should be unchanged: got %d", dst.Age)
	}
	if dst.Email != "new@example.com" {
		t.Errorf("Email should be updated: got %q", dst.Email)
	}
}

func TestMerge_AllFieldsNonZero(t *testing.T) {
	type User struct {
		Name string
		Age  int
	}

	dst := User{Name: "Alice", Age: 30}
	patch := User{Name: "Bob", Age: 25}

	if err := Merge(&dst, &patch); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if dst.Name != "Bob" {
		t.Errorf("Name should be updated: got %q", dst.Name)
	}
	if dst.Age != 25 {
		t.Errorf("Age should be updated: got %d", dst.Age)
	}
}

func TestMerge_AllFieldsZero(t *testing.T) {
	type User struct {
		Name string
		Age  int
	}

	dst := User{Name: "Alice", Age: 30}
	patch := User{} // all zero

	if err := Merge(&dst, &patch); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Nothing should change.
	if dst.Name != "Alice" {
		t.Errorf("Name should be unchanged: got %q", dst.Name)
	}
	if dst.Age != 30 {
		t.Errorf("Age should be unchanged: got %d", dst.Age)
	}
}

func TestMerge_ZeroIntIsSkipped(t *testing.T) {
	type Config struct {
		Timeout int
		Retries int
	}

	dst := Config{Timeout: 30, Retries: 3}
	patch := Config{Timeout: 60} // Retries is zero — should not overwrite

	if err := Merge(&dst, &patch); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if dst.Timeout != 60 {
		t.Errorf("Timeout should be updated: got %d", dst.Timeout)
	}
	if dst.Retries != 3 {
		t.Errorf("Retries should be unchanged: got %d", dst.Retries)
	}
}

func TestMerge_ZeroBoolIsSkipped(t *testing.T) {
	type Flags struct {
		Enabled bool
		Debug   bool
	}

	dst := Flags{Enabled: true, Debug: true}
	patch := Flags{} // both false (zero) — should not overwrite

	if err := Merge(&dst, &patch); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if !dst.Enabled {
		t.Error("Enabled should be unchanged (true)")
	}
	if !dst.Debug {
		t.Error("Debug should be unchanged (true)")
	}
}

func TestMerge_PointerField(t *testing.T) {
	type Inner struct{ X int }
	type Outer struct {
		A *Inner
		B *Inner
	}

	inner := &Inner{X: 99}
	dst := Outer{A: &Inner{X: 1}, B: &Inner{X: 2}}
	patch := Outer{A: inner} // B is nil (zero) — should not overwrite

	if err := Merge(&dst, &patch); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if dst.A == nil || dst.A.X != 99 {
		t.Errorf("A should be updated: got %v", dst.A)
	}
	if dst.B == nil || dst.B.X != 2 {
		t.Errorf("B should be unchanged: got %v", dst.B)
	}
}

func TestMerge_DifferentTypes(t *testing.T) {
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

	dst := Dst{Name: "Alice", Age: 30, City: "NYC"}
	patch := Src{City: "LA"} // only City non-zero

	if err := Merge(&dst, &patch); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if dst.Name != "Alice" {
		t.Errorf("Name should be unchanged: got %q", dst.Name)
	}
	if dst.Age != 30 {
		t.Errorf("Age should be unchanged: got %d", dst.Age)
	}
	if dst.City != "LA" {
		t.Errorf("City should be updated: got %q", dst.City)
	}
}

func TestWithSkipZeroFields_DirectOption(t *testing.T) {
	type Item struct {
		Name  string
		Score int
	}

	dst := Item{Name: "original", Score: 100}
	src := Item{Name: "updated"} // Score is zero

	if err := Copy(&dst, &src, WithSkipZeroFields(true)); err != nil {
		t.Fatalf("Copy with WithSkipZeroFields failed: %v", err)
	}

	if dst.Name != "updated" {
		t.Errorf("Name should be updated: got %q", dst.Name)
	}
	if dst.Score != 100 {
		t.Errorf("Score should be unchanged: got %d", dst.Score)
	}
}

// ── CopyError ─────────────────────────────────────────────────────────────────

func TestCopyError_ErrorsIs(t *testing.T) {
	type Src struct{ Ch chan int }
	type Dst struct{ Ch func() } // incompatible type

	src := Src{Ch: make(chan int)}
	var dst Dst

	err := Copy(&dst, &src)
	if err == nil {
		t.Fatal("expected error for incompatible types, got nil")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected errors.Is(err, ErrTypeNonCopyable), got: %v", err)
	}
}

func TestCopyError_ErrorsAs_FieldContext(t *testing.T) {
	// Force a field-level type mismatch: chan int field in src, func() field in dst.
	type Src struct{ Value chan int }
	type Dst struct{ Value func() }

	src := Src{Value: make(chan int)}
	var dst Dst

	err := Copy(&dst, &src)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *CopyError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *CopyError, got %T: %v", err, err)
	}
	if ce.SrcField == "" && ce.DstField == "" {
		t.Error("expected field context in CopyError, got empty fields")
	}
	if !errors.Is(ce, ErrTypeNonCopyable) {
		t.Errorf("CopyError.Unwrap should return ErrTypeNonCopyable")
	}
}

func TestCopyError_TopLevel_NoFieldContext(t *testing.T) {
	// Top-level type mismatch (not inside a struct field).
	var dst int
	src := "hello"

	err := Copy(&dst, &src)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrTypeNonCopyable) {
		t.Errorf("expected ErrTypeNonCopyable, got: %v", err)
	}
}

func TestCopyError_ErrorMessage(t *testing.T) {
	type Src struct{ Value chan int }
	type Dst struct{ Value func() }

	src := Src{Value: make(chan int)}
	var dst Dst

	err := Copy(&dst, &src)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if msg == "" {
		t.Error("CopyError.Error() returned empty string")
	}
}
