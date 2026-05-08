package fastcopier

import (
	"testing"
)

type SimpleStruct struct {
	Name   string
	Age    int
	Email  string
	Score  float64
	Active bool
}

type NestedStruct struct {
	ID      int
	Profile SimpleStruct
	Tags    []string
}

type ComplexStruct struct {
	ID       int
	Name     string
	Nested   NestedStruct
	Items    []SimpleStruct
	Metadata map[string]string
}

func TestCopySimpleStruct(t *testing.T) {
	src := SimpleStruct{
		Name:   "John Doe",
		Age:    30,
		Email:  "john@example.com",
		Score:  95.5,
		Active: true,
	}

	var dst SimpleStruct
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Name != src.Name {
		t.Errorf("Name mismatch: got %s, want %s", dst.Name, src.Name)
	}
	if dst.Age != src.Age {
		t.Errorf("Age mismatch: got %d, want %d", dst.Age, src.Age)
	}
	if dst.Email != src.Email {
		t.Errorf("Email mismatch: got %s, want %s", dst.Email, src.Email)
	}
	if dst.Score != src.Score {
		t.Errorf("Score mismatch: got %f, want %f", dst.Score, src.Score)
	}
	if dst.Active != src.Active {
		t.Errorf("Active mismatch: got %t, want %t", dst.Active, src.Active)
	}
}

func TestCopyNestedStruct(t *testing.T) {
	src := NestedStruct{
		ID: 1,
		Profile: SimpleStruct{
			Name:   "Jane Doe",
			Age:    25,
			Email:  "jane@example.com",
			Score:  88.0,
			Active: true,
		},
		Tags: []string{"golang", "performance", "testing"},
	}

	var dst NestedStruct
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.ID != src.ID {
		t.Errorf("ID mismatch: got %d, want %d", dst.ID, src.ID)
	}
	if dst.Profile.Name != src.Profile.Name {
		t.Errorf("Profile.Name mismatch: got %s, want %s", dst.Profile.Name, src.Profile.Name)
	}
	if len(dst.Tags) != len(src.Tags) {
		t.Errorf("Tags length mismatch: got %d, want %d", len(dst.Tags), len(src.Tags))
	}
	for i, tag := range src.Tags {
		if dst.Tags[i] != tag {
			t.Errorf("Tags[%d] mismatch: got %s, want %s", i, dst.Tags[i], tag)
		}
	}
}

func TestCopyComplexStruct(t *testing.T) {
	src := ComplexStruct{
		ID:   100,
		Name: "Complex Test",
		Nested: NestedStruct{
			ID: 1,
			Profile: SimpleStruct{
				Name:  "Nested Profile",
				Age:   35,
				Email: "nested@example.com",
			},
			Tags: []string{"tag1", "tag2"},
		},
		Items: []SimpleStruct{
			{Name: "Item1", Age: 10},
			{Name: "Item2", Age: 20},
		},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	var dst ComplexStruct
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
	if dst.Nested.Profile.Name != src.Nested.Profile.Name {
		t.Errorf("Nested profile name mismatch")
	}
	if len(dst.Items) != len(src.Items) {
		t.Errorf("Items length mismatch")
	}
	if len(dst.Metadata) != len(src.Metadata) {
		t.Errorf("Metadata length mismatch")
	}
}

func TestCopyNilPointer(t *testing.T) {
	var src *SimpleStruct
	var dst SimpleStruct

	err := Copy(&dst, src)
	if err == nil {
		t.Error("Expected error for nil src, got nil")
	}
}

func TestCopyNonPointer(t *testing.T) {
	src := SimpleStruct{Name: "Test"}
	dst := SimpleStruct{}

	err := Copy(dst, src)
	if err == nil {
		t.Error("Expected error for non-pointer arguments, got nil")
	}
}

func TestCopyTypeConversion(t *testing.T) {
	type Source struct {
		Age   int32
		Score float32
	}

	type Destination struct {
		Age   int64
		Score float64
	}

	src := Source{Age: 30, Score: 95.5}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Age != int64(src.Age) {
		t.Errorf("Age conversion failed: got %d, want %d", dst.Age, int64(src.Age))
	}
	if dst.Score != float64(src.Score) {
		t.Errorf("Score conversion failed: got %f, want %f", dst.Score, float64(src.Score))
	}
}

func TestCopyPartialFields(t *testing.T) {
	type Source struct {
		Name  string
		Age   int
		Email string
	}

	type Destination struct {
		Name string
		Age  int
	}

	src := Source{Name: "John", Age: 30, Email: "john@example.com"}
	var dst Destination

	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Name != src.Name {
		t.Errorf("Name mismatch")
	}
	if dst.Age != src.Age {
		t.Errorf("Age mismatch")
	}
}
