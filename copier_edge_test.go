package fastcopier

import (
	"strings"
	"testing"
)

// Circular Reference Tests

type Node struct {
	Value int
	Next  *Node
}

func TestCopyCircularReferenceSelfReferencing(t *testing.T) {
	src := &Node{Value: 1}
	src.Next = src // Self-referencing

	var dst Node
	err := Copy(&dst, src)
	if err == nil {
		t.Fatal("Expected circular reference error, got nil")
	}
	if !strings.Contains(err.Error(), "circular reference") {
		t.Errorf("Expected circular reference error, got: %v", err)
	}
}

func TestCopyCircularReferenceMutual(t *testing.T) {
	// Create a circular reference through a chain
	node1 := &Node{Value: 1}
	node2 := &Node{Value: 2}
	node3 := &Node{Value: 3}

	node1.Next = node2
	node2.Next = node3
	node3.Next = node1 // Circular back to node1

	var dst Node
	err := Copy(&dst, node1)
	if err == nil {
		t.Error("Expected circular reference error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "circular reference") {
		t.Errorf("Expected circular reference error, got: %v", err)
	}
}

func TestCopyCircularReferenceInSlice(t *testing.T) {
	src := &Node{Value: 1}
	src.Next = &Node{Value: 2}
	src.Next.Next = src // Circular through slice

	var dst Node
	err := Copy(&dst, src)
	if err == nil {
		t.Error("Expected circular reference error, got nil")
	}
}

// Embedded Struct Tests

func TestCopyEmbeddedStruct(t *testing.T) {
	type Base struct {
		ID   int
		Name string
	}

	type Source struct {
		Base
		Email string
	}

	type Destination struct {
		ID    int
		Name  string
		Email string
	}

	src := Source{
		Base:  Base{ID: 1, Name: "John"},
		Email: "john@example.com",
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.ID != src.ID {
		t.Errorf("ID mismatch: got %d, want %d", dst.ID, src.ID)
	}
	if dst.Name != src.Name {
		t.Errorf("Name mismatch: got %s, want %s", dst.Name, src.Name)
	}
	if dst.Email != src.Email {
		t.Errorf("Email mismatch")
	}
}

func TestCopyEmbeddedStructBothSides(t *testing.T) {
	type Base struct {
		ID   int
		Name string
	}

	type Source struct {
		Base
		Email string
	}

	type Destination struct {
		Base
		Email string
	}

	src := Source{
		Base:  Base{ID: 1, Name: "John"},
		Email: "john@example.com",
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.ID != src.ID {
		t.Errorf("ID mismatch")
	}
	if dst.Name != src.Name {
		t.Errorf("Name mismatch")
	}
	if dst.Email != src.Email {
		t.Errorf("Email mismatch")
	}
}

func TestCopyEmbeddedStructNameCollision(t *testing.T) {
	type Base struct {
		Name string
	}

	type Source struct {
		Base
		Name string // Shadows embedded Name
	}

	type Destination struct {
		Name string
	}

	src := Source{
		Base: Base{Name: "Embedded"},
		Name: "Outer",
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Outer field should take precedence
	if dst.Name != "Outer" {
		t.Errorf("Name should be 'Outer', got %s", dst.Name)
	}
}

// Interface Tests

func TestCopyInterfaceField(t *testing.T) {
	type Source struct {
		Value interface{}
	}

	type Destination struct {
		Value interface{}
	}

	src := Source{Value: "test string"}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Value != src.Value {
		t.Errorf("Value mismatch: got %v, want %v", dst.Value, src.Value)
	}
}

func TestCopyNilInterface(t *testing.T) {
	type Source struct {
		Value interface{}
	}

	type Destination struct {
		Value interface{}
	}

	src := Source{Value: nil}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Value != nil {
		t.Errorf("Value should be nil, got %v", dst.Value)
	}
}

func TestCopyInterfaceWithStruct(t *testing.T) {
	type Inner struct {
		Name string
	}

	type Source struct {
		Value interface{}
	}

	type Destination struct {
		Value interface{}
	}

	src := Source{Value: Inner{Name: "test"}}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	inner, ok := dst.Value.(Inner)
	if !ok {
		t.Errorf("Value should be Inner type")
	}
	if inner.Name != "test" {
		t.Errorf("Inner.Name mismatch")
	}
}

// Channel Tests

func TestCopyChannelField(t *testing.T) {
	type Source struct {
		Ch chan int
	}

	type Destination struct {
		Ch chan int
	}

	ch := make(chan int, 1)
	ch <- 42

	src := Source{Ch: ch}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Channels should be copied by reference
	if dst.Ch != src.Ch {
		t.Error("Channel should be copied by reference")
	}

	// Verify it's the same channel
	val := <-dst.Ch
	if val != 42 {
		t.Errorf("Channel value mismatch: got %d, want 42", val)
	}
}

func TestCopyNilChannel(t *testing.T) {
	type Source struct {
		Ch chan int
	}

	type Destination struct {
		Ch chan int
	}

	src := Source{Ch: nil}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Ch != nil {
		t.Error("Channel should be nil")
	}
}

// Function Tests

func TestCopyFunctionField(t *testing.T) {
	type Source struct {
		Fn func(int) int
	}

	type Destination struct {
		Fn func(int) int
	}

	fn := func(x int) int { return x * 2 }
	src := Source{Fn: fn}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Functions should be copied by reference
	if dst.Fn == nil {
		t.Error("Function should not be nil")
	}

	result := dst.Fn(5)
	if result != 10 {
		t.Errorf("Function result mismatch: got %d, want 10", result)
	}
}

func TestCopyNilFunction(t *testing.T) {
	type Source struct {
		Fn func()
	}

	type Destination struct {
		Fn func()
	}

	src := Source{Fn: nil}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Fn != nil {
		t.Error("Function should be nil")
	}
}

// Complex Pointer Tests

func TestCopyPointerToPointer(t *testing.T) {
	type Source struct {
		Value **int
	}

	type Destination struct {
		Value **int
	}

	val := 42
	ptr := &val
	src := Source{Value: &ptr}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Value == nil {
		t.Error("Value should not be nil")
	}
	if *dst.Value == nil {
		t.Error("*Value should not be nil")
	}
	if **dst.Value != 42 {
		t.Errorf("**Value mismatch: got %d, want 42", **dst.Value)
	}
}

func TestCopyPointerToSlice(t *testing.T) {
	type Source struct {
		Items *[]int
	}

	type Destination struct {
		Items *[]int
	}

	items := []int{1, 2, 3}
	src := Source{Items: &items}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Items == nil {
		t.Error("Items should not be nil")
	}
	if len(*dst.Items) != 3 {
		t.Errorf("Items length mismatch: got %d, want 3", len(*dst.Items))
	}
}

func TestCopyPointerToMap(t *testing.T) {
	type Source struct {
		Data *map[string]int
	}

	type Destination struct {
		Data *map[string]int
	}

	data := map[string]int{"a": 1, "b": 2}
	src := Source{Data: &data}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Data == nil {
		t.Error("Data should not be nil")
	}
	if len(*dst.Data) != 2 {
		t.Errorf("Data length mismatch: got %d, want 2", len(*dst.Data))
	}
}

func TestCopyNilPointerToPointer(t *testing.T) {
	type Source struct {
		Value **int
	}

	type Destination struct {
		Value **int
	}

	src := Source{Value: nil}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Value != nil {
		t.Error("Value should be nil")
	}
}

// Mixed Complex Tests

func TestCopySliceOfPointers(t *testing.T) {
	type Source struct {
		Items []*int
	}

	type Destination struct {
		Items []*int
	}

	val1, val2 := 1, 2
	src := Source{Items: []*int{&val1, &val2}}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if len(dst.Items) != 2 {
		t.Errorf("Items length mismatch")
	}
	if *dst.Items[0] != 1 || *dst.Items[1] != 2 {
		t.Error("Items values mismatch")
	}
}

func TestCopyMapWithPointerValues(t *testing.T) {
	type Source struct {
		Data map[string]*int
	}

	type Destination struct {
		Data map[string]*int
	}

	val1, val2 := 10, 20
	src := Source{Data: map[string]*int{"a": &val1, "b": &val2}}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if len(dst.Data) != 2 {
		t.Errorf("Data length mismatch")
	}
	if *dst.Data["a"] != 10 || *dst.Data["b"] != 20 {
		t.Error("Data values mismatch")
	}
}
