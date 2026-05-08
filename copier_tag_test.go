package fastcopier

import (
	"testing"
)

// Test basic struct tag mapping
func TestCopyWithStructTags(t *testing.T) {
	type Source struct {
		UserName string `fastcopier:"Name"`
		UserAge  int    `fastcopier:"Age"`
		Email    string
	}

	type Destination struct {
		Name  string
		Age   int
		Email string
	}

	src := Source{
		UserName: "John Doe",
		UserAge:  30,
		Email:    "john@example.com",
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Name != src.UserName {
		t.Errorf("Name mismatch: got %s, want %s", dst.Name, src.UserName)
	}
	if dst.Age != src.UserAge {
		t.Errorf("Age mismatch: got %d, want %d", dst.Age, src.UserAge)
	}
	if dst.Email != src.Email {
		t.Errorf("Email mismatch: got %s, want %s", dst.Email, src.Email)
	}
}

// Test skip field with - tag
func TestCopySkipFieldWithTag(t *testing.T) {
	type Source struct {
		Name     string
		Password string `fastcopier:"-"`
		Email    string
	}

	type Destination struct {
		Name     string
		Password string
		Email    string
	}

	src := Source{
		Name:     "John Doe",
		Password: "secret123",
		Email:    "john@example.com",
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Name != src.Name {
		t.Errorf("Name mismatch")
	}
	if dst.Password != "" {
		t.Errorf("Password should be empty (skipped), got %s", dst.Password)
	}
	if dst.Email != src.Email {
		t.Errorf("Email mismatch")
	}
}

// Test tag with missing target field
func TestCopyTagWithMissingTarget(t *testing.T) {
	type Source struct {
		Name    string `fastcopier:"FullName"`
		Age     int
		Unknown string `fastcopier:"NonExistent"`
	}

	type Destination struct {
		Name string
		Age  int
	}

	src := Source{
		Name:    "John Doe",
		Age:     30,
		Unknown: "should be ignored",
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Name should not be copied because tag points to non-existent field
	if dst.Name != "" {
		t.Errorf("Name should be empty, got %s", dst.Name)
	}
	if dst.Age != src.Age {
		t.Errorf("Age mismatch")
	}
}

// Test tag priority over field name
func TestCopyTagPriorityOverFieldName(t *testing.T) {
	type Source struct {
		Name  string `fastcopier:"Title"`
		Title string
	}

	type Destination struct {
		Name  string
		Title string
	}

	src := Source{
		Name:  "John Doe",
		Title: "Engineer",
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Name field should map to Title via tag
	if dst.Title != src.Name {
		t.Errorf("Title should be %s (from Name field via tag), got %s", src.Name, dst.Title)
	}
	// Name should remain empty since no source field maps to it
	if dst.Name != "" {
		t.Errorf("Name should be empty, got %s", dst.Name)
	}
}

// Test multiple fields mapping to same destination
func TestCopyMultipleFieldsToSameDestination(t *testing.T) {
	type Source struct {
		FirstName string `fastcopier:"Name"`
		LastName  string
	}

	type Destination struct {
		Name     string
		LastName string
	}

	src := Source{
		FirstName: "John",
		LastName:  "Doe",
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// FirstName should map to Name via tag
	if dst.Name != src.FirstName {
		t.Errorf("Name should be %s, got %s", src.FirstName, dst.Name)
	}
	if dst.LastName != src.LastName {
		t.Errorf("LastName mismatch")
	}
}

// Test tag with type conversion
func TestCopyTagWithTypeConversion(t *testing.T) {
	type Source struct {
		UserAge int32 `fastcopier:"Age"`
	}

	type Destination struct {
		Age int64
	}

	src := Source{
		UserAge: 30,
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Age != int64(src.UserAge) {
		t.Errorf("Age conversion failed: got %d, want %d", dst.Age, int64(src.UserAge))
	}
}

// Test nested struct with tags
func TestCopyNestedStructWithTags(t *testing.T) {
	type Address struct {
		StreetName string `fastcopier:"Street"`
		CityName   string `fastcopier:"City"`
	}

	type Person struct {
		PersonName string  `fastcopier:"Name"`
		Location   Address `fastcopier:"Address"`
	}

	type DestAddress struct {
		Street string
		City   string
	}

	type DestPerson struct {
		Name    string
		Address DestAddress
	}

	src := Person{
		PersonName: "John Doe",
		Location: Address{
			StreetName: "Main St",
			CityName:   "New York",
		},
	}

	var dst DestPerson
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Name != src.PersonName {
		t.Errorf("Name mismatch")
	}
	if dst.Address.Street != src.Location.StreetName {
		t.Errorf("Street mismatch")
	}
	if dst.Address.City != src.Location.CityName {
		t.Errorf("City mismatch")
	}
}

// Test empty tag (should fall back to field name)
func TestCopyEmptyTag(t *testing.T) {
	type Source struct {
		Name string `fastcopier:""`
		Age  int
	}

	type Destination struct {
		Name string
		Age  int
	}

	src := Source{
		Name: "John Doe",
		Age:  30,
	}

	var dst Destination
	err := Copy(&dst, &src)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if dst.Name != src.Name {
		t.Errorf("Name should match via field name fallback")
	}
	if dst.Age != src.Age {
		t.Errorf("Age mismatch")
	}
}
