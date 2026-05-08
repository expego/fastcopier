package fastcopier_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/expego/fastcopier"
)

// ── Copy ──────────────────────────────────────────────────────────────────────

func ExampleCopy_structs() {
	type Source struct {
		Name  string
		Age   int
		Email string
	}
	type Destination struct {
		Name  string
		Age   int
		Email string
	}

	src := Source{Name: "Alice", Age: 30, Email: "alice@example.com"}
	var dst Destination

	if err := fastcopier.Copy(&dst, &src); err != nil {
		log.Fatal(err)
	}

	fmt.Println(dst.Name, dst.Age, dst.Email)
	// Output: Alice 30 alice@example.com
}

func ExampleCopy_slices() {
	type Src struct{ Name string }
	type Dst struct{ Name string }

	src := []Src{{Name: "Alice"}, {Name: "Bob"}}
	var dst []Dst

	if err := fastcopier.Copy(&dst, &src); err != nil {
		log.Fatal(err)
	}

	fmt.Println(len(dst), dst[0].Name, dst[1].Name)
	// Output: 2 Alice Bob
}

func ExampleCopy_maps() {
	src := map[string]int{"a": 1, "b": 2}
	var dst map[string]int

	if err := fastcopier.Copy(&dst, &src); err != nil {
		log.Fatal(err)
	}

	fmt.Println(len(dst))
	// Output: 2
}

func ExampleCopy_withTags() {
	type Source struct {
		UserName string `fastcopier:"Name"`
		Internal string `fastcopier:"-"` // skip
	}
	type Destination struct {
		Name     string
		Internal string
	}

	src := Source{UserName: "Bob", Internal: "secret"}
	var dst Destination

	if err := fastcopier.Copy(&dst, &src); err != nil {
		log.Fatal(err)
	}

	fmt.Println(dst.Name, dst.Internal)
	// Output: Bob
}

// ── Clone ─────────────────────────────────────────────────────────────────────

func ExampleClone() {
	type Config struct {
		Host string
		Port int
	}

	original := Config{Host: "localhost", Port: 8080}
	cloned, err := fastcopier.Clone(original)
	if err != nil {
		log.Fatal(err)
	}

	// Mutating the clone does not affect the original.
	cloned.Host = "remote"
	fmt.Println(original.Host, cloned.Host)
	// Output: localhost remote
}

// ── Map ───────────────────────────────────────────────────────────────────────

func ExampleMap() {
	type UserEntity struct {
		ID    int
		Name  string
		Email string
	}
	type UserDTO struct {
		ID   int
		Name string
	}

	entities := []UserEntity{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
	}

	dtos, err := fastcopier.Map[UserEntity, UserDTO](entities)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(len(dtos), dtos[0].Name, dtos[1].Name)
	// Output: 2 Alice Bob
}

// ── Merge ─────────────────────────────────────────────────────────────────────

func ExampleMerge() {
	type User struct {
		Name  string
		Age   int
		Email string
	}

	existing := User{Name: "Alice", Age: 30, Email: "alice@example.com"}
	patch := User{Email: "new@example.com"} // only Email is non-zero

	if err := fastcopier.Merge(&existing, &patch); err != nil {
		log.Fatal(err)
	}

	// Name and Age are unchanged; only Email was updated.
	fmt.Println(existing.Name, existing.Age, existing.Email)
	// Output: Alice 30 new@example.com
}

// ── Inspect ───────────────────────────────────────────────────────────────────

func ExampleInspect() {
	type UserEntity struct {
		ID    int
		Name  string
		Email string // no matching field in DTO → will be skipped
	}
	type UserDTO struct {
		ID   int
		Name string
	}

	plan, err := fastcopier.Inspect(&UserDTO{}, &UserEntity{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(len(plan.Fields))   // 2 fields copied: ID, Name
	fmt.Println(len(plan.Skipped))  // 1 skipped: Email
	// Output:
	// 2
	// 1
}

// ── MustRegister ──────────────────────────────────────────────────────────────

func ExampleMustRegister() {
	type UserEntity struct{ Name string }
	type UserDTO struct{ Name string }

	// Call at program startup to catch mapping errors early.
	// Panics if the plan cannot be built (e.g. incompatible required fields).
	fastcopier.MustRegister(&UserDTO{}, &UserEntity{})
	fmt.Println("registered")
	// Output: registered
}

// ── WithFields ────────────────────────────────────────────────────────────────

func ExampleWithFields() {
	type User struct {
		Name  string
		Age   int
		Email string
	}

	src := User{Name: "Alice", Age: 30, Email: "alice@example.com"}
	dst := User{Name: "old", Age: 99, Email: "old@example.com"}

	// Only copy the Name field; Age and Email are left untouched.
	if err := fastcopier.Copy(&dst, &src, fastcopier.WithFields("Name")); err != nil {
		log.Fatal(err)
	}

	fmt.Println(dst.Name, dst.Age, dst.Email)
	// Output: Alice 99 old@example.com
}

// ── WithSkipZeroFields ────────────────────────────────────────────────────────

func ExampleWithSkipZeroFields() {
	type Config struct {
		Timeout int
		Retries int
	}

	dst := Config{Timeout: 30, Retries: 3}
	src := Config{Timeout: 60} // Retries is zero — should not overwrite

	if err := fastcopier.Copy(&dst, &src, fastcopier.WithSkipZeroFields(true)); err != nil {
		log.Fatal(err)
	}

	fmt.Println(dst.Timeout, dst.Retries)
	// Output: 60 3
}

// ── CopyError ─────────────────────────────────────────────────────────────────

func ExampleCopyError() {
	type Src struct{ Value chan int }
	type Dst struct{ Value func() } // incompatible

	src := Src{Value: make(chan int)}
	var dst Dst

	err := fastcopier.Copy(&dst, &src)

	var ce *fastcopier.CopyError
	if errors.As(err, &ce) {
		fmt.Println(errors.Is(ce, fastcopier.ErrTypeNonCopyable))
	}
	// Output: true
}
