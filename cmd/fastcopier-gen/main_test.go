package main

import (
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── splitComma ────────────────────────────────────────────────────────────────

func TestSplitComma(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"Foo", []string{"Foo"}},
		{"Foo,Bar", []string{"Foo", "Bar"}},
		{"Foo , Bar , Baz", []string{"Foo", "Bar", "Baz"}},
		{",", nil},         // empty segments are dropped
		{"  ,Foo,  ", []string{"Foo"}},
	}
	for _, tc := range tests {
		got := splitComma(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitComma(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i, g := range got {
			if g != tc.want[i] {
				t.Errorf("splitComma(%q)[%d] = %q, want %q", tc.in, i, g, tc.want[i])
			}
		}
	}
}

// ── isFlatGoType ──────────────────────────────────────────────────────────────

func TestIsFlatGoType(t *testing.T) {
	intType := types.Typ[types.Int]
	strType := types.Typ[types.String]

	tests := []struct {
		name string
		t    types.Type
		want bool
	}{
		// Basics are flat.
		{"int", intType, true},
		{"string", strType, true},
		{"bool", types.Typ[types.Bool], true},
		{"float64", types.Typ[types.Float64], true},
		// Function signature is flat (treated same as scalars).
		{"func()", types.NewSignatureType(nil, nil, nil, nil, nil, false), true},
		// Array of flat elements is flat.
		{"[3]int", types.NewArray(intType, 3), true},
		// Struct with all flat fields is flat.
		{"struct{X int}", types.NewStruct([]*types.Var{
			types.NewVar(0, nil, "X", intType),
		}, nil), true},
		// Struct with a non-flat field is not flat.
		{"struct{S []int}", types.NewStruct([]*types.Var{
			types.NewVar(0, nil, "S", types.NewSlice(intType)),
		}, nil), false},
		// Slice, Map, Pointer, Interface, Chan are not flat.
		{"[]int", types.NewSlice(intType), false},
		{"map[string]int", types.NewMap(strType, intType), false},
		{"*int", types.NewPointer(intType), false},
		{"interface{}", types.NewInterfaceType(nil, nil), false},
		{"chan int", types.NewChan(types.SendRecv, intType), false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := isFlatGoType(tc.t); got != tc.want {
				t.Errorf("isFlatGoType(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// ── classifyType ──────────────────────────────────────────────────────────────

func TestClassifyType(t *testing.T) {
	intType := types.Typ[types.Int]
	strType := types.Typ[types.String]
	flatStruct := types.NewStruct([]*types.Var{
		types.NewVar(0, nil, "X", intType),
	}, nil)
	nonFlatStruct := types.NewStruct([]*types.Var{
		types.NewVar(0, nil, "S", types.NewSlice(intType)),
	}, nil)

	tests := []struct {
		name string
		t    types.Type
		want typeKind
	}{
		// Flat types → kindFlat.
		{"int", intType, kindFlat},
		{"string", strType, kindFlat},
		{"flat struct", flatStruct, kindFlat},
		// Slice of flat → kindSliceFlat.
		{"[]int", types.NewSlice(intType), kindSliceFlat},
		// Slice of non-flat struct → kindSliceStruct.
		{"[]nonFlatStruct", types.NewSlice(nonFlatStruct), kindSliceStruct},
		// Slice of slice (non-flat, non-struct) → kindFallback.
		{"[][]int", types.NewSlice(types.NewSlice(intType)), kindFallback},
		// Map with flat key and value → kindMapFlat.
		{"map[string]int", types.NewMap(strType, intType), kindMapFlat},
		// Map with non-flat value → kindMapComplex.
		{"map[string][]int", types.NewMap(strType, types.NewSlice(intType)), kindMapComplex},
		// Pointer → kindPointer.
		{"*int", types.NewPointer(intType), kindPointer},
		// Non-flat struct → kindStruct.
		{"non-flat struct", nonFlatStruct, kindStruct},
		// Chan → kindFallback.
		{"chan int", types.NewChan(types.SendRecv, intType), kindFallback},
		// Interface → kindFallback.
		{"interface{}", types.NewInterfaceType(nil, nil), kindFallback},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyType(tc.t); got != tc.want {
				t.Errorf("classifyType(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// ── typeName ──────────────────────────────────────────────────────────────────

func TestTypeName(t *testing.T) {
	// Named type.
	pkg := types.NewPackage("example.com/foo", "foo")
	obj := types.NewTypeName(0, pkg, "MyStruct", nil)
	named := types.NewNamed(obj, types.NewStruct(nil, nil), nil)
	if got := typeName(named); got != "MyStruct" {
		t.Errorf("typeName(Named) = %q, want %q", got, "MyStruct")
	}

	// Pointer to named.
	ptr := types.NewPointer(named)
	if got := typeName(ptr); got != "MyStruct" {
		t.Errorf("typeName(*Named) = %q, want %q", got, "MyStruct")
	}

	// Unnamed type (e.g. []int) — TypeString without package qualifier.
	sl := types.NewSlice(types.Typ[types.Int])
	if got := typeName(sl); got != "[]int" {
		t.Errorf("typeName([]int) = %q, want %q", got, "[]int")
	}
}

// ── loadPackage + lookupStruct + matchFields integration ─────────────────────

// makeTmpPkg writes src as "types.go" in a temp dir and returns the dir path.
func makeTmpPkg(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return dir
}

func TestLoadPackage_Basic(t *testing.T) {
	dir := makeTmpPkg(t, `package mypkg

type Foo struct {
	X int
	Y string
}
`)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}
	if pkg.name != "mypkg" {
		t.Errorf("pkg.name = %q, want %q", pkg.name, "mypkg")
	}
}

func TestLoadPackage_NoPackage(t *testing.T) {
	// Empty dir → no non-test package.
	dir := t.TempDir()
	_, err := loadPackage(dir)
	if err == nil {
		t.Fatal("expected error for empty dir, got nil")
	}
}

func TestLookupStruct_Found(t *testing.T) {
	dir := makeTmpPkg(t, `package mypkg

type Foo struct{ X int }
type Bar struct{ Y string }
`)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}
	named := lookupStruct(pkg.scope, "Foo")
	if named == nil {
		t.Fatal("lookupStruct(Foo) returned nil")
	}
	if named.Obj().Name() != "Foo" {
		t.Errorf("name = %q, want Foo", named.Obj().Name())
	}
}

func TestLookupStruct_NotFound(t *testing.T) {
	dir := makeTmpPkg(t, `package mypkg

type Foo struct{ X int }
`)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}
	if got := lookupStruct(pkg.scope, "NoSuchType"); got != nil {
		t.Errorf("lookupStruct(NoSuchType) = %v, want nil", got)
	}
}

func TestLookupStruct_NotAStruct(t *testing.T) {
	dir := makeTmpPkg(t, `package mypkg

type MyInt int
`)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}
	// MyInt is a Named type but not a struct — should return nil.
	if got := lookupStruct(pkg.scope, "MyInt"); got != nil {
		t.Errorf("lookupStruct(MyInt) = %v, want nil", got)
	}
}

func TestMatchFields_SameType(t *testing.T) {
	dir := makeTmpPkg(t, `package mypkg

type Foo struct {
	Exported   int
	unexported string
}
`)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}
	foo := lookupStruct(pkg.scope, "Foo")
	fps := matchFields(foo, foo)
	// Only exported field "Exported" should be matched.
	if len(fps) != 1 {
		t.Fatalf("matchFields same-type: got %d pairs, want 1", len(fps))
	}
	if fps[0].DstField.Name() != "Exported" {
		t.Errorf("pair[0] = %q, want Exported", fps[0].DstField.Name())
	}
}

func TestMatchFields_CrossType_PartialOverlap(t *testing.T) {
	dir := makeTmpPkg(t, `package mypkg

type Src struct {
	Shared  int
	SrcOnly string
}
type Dst struct {
	Shared  int
	DstOnly float64
}
`)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}
	src := lookupStruct(pkg.scope, "Src")
	dst := lookupStruct(pkg.scope, "Dst")
	fps := matchFields(dst, src)
	if len(fps) != 1 {
		t.Fatalf("matchFields cross-type: got %d pairs, want 1", len(fps))
	}
	if fps[0].DstField.Name() != "Shared" {
		t.Errorf("pair[0] = %q, want Shared", fps[0].DstField.Name())
	}
}

// ── generate integration tests ────────────────────────────────────────────────

// testGenerate is a helper that generates code for the given Go source and
// returns the output as a string.
func testGenerate(t *testing.T, src string, typeNames ...string) string {
	t.Helper()
	dir := makeTmpPkg(t, src)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}
	return testGenerateFromPkg(t, pkg, typeNames...)
}

// testGenerateFromPkg generates code from a pre-loaded package.
func testGenerateFromPkg(t *testing.T, pkg *pkgInfo, typeNames ...string) string {
	t.Helper()
	var pairs []typePair
	for _, name := range typeNames {
		tp := lookupStruct(pkg.scope, name)
		if tp == nil {
			t.Fatalf("type %q not found", name)
		}
		pairs = append(pairs, typePair{Dst: tp, Src: tp, SameType: true})
	}

	out, err := generate(pkg, pairs)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return string(out)
}

func TestGenerate_FlatFields(t *testing.T) {
	out := testGenerate(t, `package mypkg

type Simple struct {
	X int
	Y string
	Z float64
}
`, "Simple")

	// Flat fields use direct assignment.
	if !strings.Contains(out, "dst.X = src.X") {
		t.Error("expected 'dst.X = src.X' in generated code")
	}
	if !strings.Contains(out, "dst.Y = src.Y") {
		t.Error("expected 'dst.Y = src.Y' in generated code")
	}
	// Function should be registered.
	if !strings.Contains(out, "RegisterCopier") {
		t.Error("expected RegisterCopier call in generated code")
	}
}

func TestGenerate_SliceFlatField(t *testing.T) {
	out := testGenerate(t, `package mypkg

type WithSlice struct {
	Items []int
}
`, "WithSlice")

	// Flat slice uses builtin copy.
	if !strings.Contains(out, "copy(dst.Items, src.Items)") {
		t.Errorf("expected copy() call for flat slice, got:\n%s", out)
	}
}

func TestGenerate_MapFlatField(t *testing.T) {
	out := testGenerate(t, `package mypkg

type WithMap struct {
	Tags map[string]int
}
`, "WithMap")

	// Flat map uses direct value assignment in range.
	if !strings.Contains(out, "dst.Tags[k] = v") {
		t.Errorf("expected direct map value assignment, got:\n%s", out)
	}
}

func TestGenerate_MapComplexField(t *testing.T) {
	out := testGenerate(t, `package mypkg

type WithComplexMap struct {
	Data map[string][]int
}
`, "WithComplexMap")

	// Complex map value uses fastcopier fallback.
	if !strings.Contains(out, "fastcopier.Copy") {
		t.Errorf("expected fastcopier.Copy fallback for complex map, got:\n%s", out)
	}
}

func TestGenerate_PointerFlatField(t *testing.T) {
	out := testGenerate(t, `package mypkg

type WithPtr struct {
	Count *int
}
`, "WithPtr")

	// Flat pointer: dereference copy.
	if !strings.Contains(out, "v := *src.Count") {
		t.Errorf("expected dereference copy for flat pointer, got:\n%s", out)
	}
}

func TestGenerate_FallbackField(t *testing.T) {
	out := testGenerate(t, `package mypkg

type WithChan struct {
	Ch chan int
}
`, "WithChan")

	// Channel (kindFallback) uses fastcopier.Copy.
	if !strings.Contains(out, "fastcopier.Copy") {
		t.Errorf("expected fastcopier.Copy fallback for chan field, got:\n%s", out)
	}
}

func TestGenerate_SliceStructKnownType(t *testing.T) {
	out := testGenerate(t, `package mypkg

type Item struct {
	V int
}
type WithItems struct {
	Items []Item
}
`, "Item", "WithItems")

	// Slice of known struct type uses generated copy function.
	if !strings.Contains(out, "CopyItemToItem") {
		t.Errorf("expected CopyItemToItem call for []Item, got:\n%s", out)
	}
}

func TestGenerate_PointerKnownType(t *testing.T) {
	out := testGenerate(t, `package mypkg

type Child struct {
	V int
}
type Parent struct {
	Child *Child
}
`, "Child", "Parent")

	// Pointer to known struct type uses generated copy function.
	if !strings.Contains(out, "CopyChildToChild") {
		t.Errorf("expected CopyChildToChild for *Child field, got:\n%s", out)
	}
}

func TestGenerate_StructKnownType(t *testing.T) {
	out := testGenerate(t, `package mypkg

type Inner struct {
	V []int
}
type Outer struct {
	Inner Inner
}
`, "Inner", "Outer")

	// Known non-flat struct field uses generated copy function.
	if !strings.Contains(out, "CopyInnerToInner") {
		t.Errorf("expected CopyInnerToInner for Inner field, got:\n%s", out)
	}
}

func TestGenerate_CrossType(t *testing.T) {
	dir := makeTmpPkg(t, `package mypkg

type UserEntity struct {
	Name  string
	Email string
	Age   int
}
type UserDTO struct {
	Name  string
	Email string
}
`)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}

	srcTyp := lookupStruct(pkg.scope, "UserEntity")
	dstTyp := lookupStruct(pkg.scope, "UserDTO")
	pairs := []typePair{{Dst: dstTyp, Src: srcTyp, SameType: false}}

	out, err := generate(pkg, pairs)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	code := string(out)

	// Function name should reflect Src→Dst.
	if !strings.Contains(code, "CopyUserEntityToUserDTO") {
		t.Errorf("expected CopyUserEntityToUserDTO, got:\n%s", code)
	}
	// Only shared fields (Name, Email) should appear.
	if !strings.Contains(code, "dst.Name = src.Name") {
		t.Error("expected dst.Name = src.Name")
	}
	if !strings.Contains(code, "dst.Email = src.Email") {
		t.Error("expected dst.Email = src.Email")
	}
	// Age is not in UserDTO and should NOT appear.
	if strings.Contains(code, "Age") {
		t.Error("Age should not appear in cross-type output (not in dst)")
	}
}

func TestGenerate_StructUnknownType(t *testing.T) {
	// Non-flat struct field where only the outer type is generated (not the inner).
	// The field should fall back to fastcopier.Copy.
	out := testGenerate(t, `package mypkg

type Inner struct {
	Nested []int
}
type Outer struct {
	Inner Inner
}
`, "Outer") // Inner is NOT in the pairs, so it's unknown.

	if !strings.Contains(out, "fastcopier.Copy") {
		t.Errorf("expected fastcopier.Copy fallback for unknown struct field, got:\n%s", out)
	}
}

func TestGenerate_SliceStructUnknownType(t *testing.T) {
	out := testGenerate(t, `package mypkg

type Item struct {
	Tags []string
}
type WithItems struct {
	Items []Item
}
`, "WithItems") // Item is NOT in pairs → fallback copy in loop.

	if !strings.Contains(out, "fastcopier.Copy") {
		t.Errorf("expected fastcopier.Copy fallback for unknown []struct, got:\n%s", out)
	}
}

func TestGenerate_PointerUnknownStructType(t *testing.T) {
	out := testGenerate(t, `package mypkg

type Child struct {
	Tags []string
}
type Parent struct {
	Child *Child
}
`, "Parent") // Child is NOT in pairs → fall back for unknown non-flat pointer.

	if !strings.Contains(out, "fastcopier.Copy") {
		t.Errorf("expected fastcopier.Copy fallback for *UnknownStruct, got:\n%s", out)
	}
}

// TestGenerate_SliceStructNonFlatKnownType exercises sliceStructLines with a
// non-flat struct element type that IS in the known-types set.  This covers the
// "g.knownTypes[elemName] == true" branch of sliceStructLines.
func TestGenerate_SliceStructNonFlatKnownType(t *testing.T) {
	// Item has a slice field → non-flat → []Item uses kindSliceStruct.
	// Both Item and Container are in the generated set → known type path.
	out := testGenerate(t, `package mypkg

type Item struct {
	Tags []string
}
type Container struct {
	Items []Item
}
`, "Item", "Container")

	// Loop should use the generated CopyItemToItem helper, not fastcopier.
	if !strings.Contains(out, "CopyItemToItem") {
		t.Errorf("expected CopyItemToItem for non-flat []Item (known type), got:\n%s", out)
	}
}

// TestLookupStruct_FunctionName covers the !ok branch in lookupStruct when the
// looked-up name resolves to a non-Named type (e.g. a function).
func TestLookupStruct_FunctionName(t *testing.T) {
	dir := makeTmpPkg(t, `package mypkg

func Helper() {}
`)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}
	// "Helper" is a function, not a Named struct type → should return nil.
	if got := lookupStruct(pkg.scope, "Helper"); got != nil {
		t.Errorf("lookupStruct(Helper) = %v, want nil", got)
	}
}

// TestTypeStr_ExternalPackage exercises the external-package qualifier branch
// of typeStr (p.Path() != g.pkg.path → return p.Name()).
// []time.Duration is a flat slice whose element type comes from an external
// package; sliceFlatLines calls g.typeStr(t) for the make() expression.
func TestTypeStr_ExternalPackage(t *testing.T) {
	dir := makeTmpPkg(t, `package mypkg

import "time"

type WithDurations struct {
	Items []time.Duration
}
`)
	pkg, err := loadPackage(dir)
	if err != nil {
		t.Fatalf("loadPackage: %v", err)
	}
	out := testGenerateFromPkg(t, pkg, "WithDurations")
	// sliceFlatLines uses g.typeStr([]time.Duration) → "[]time.Duration".
	if !strings.Contains(out, "time.Duration") {
		t.Errorf("expected 'time.Duration' in generated code (external pkg qualifier), got:\n%s", out)
	}
}

// TestTypeName_TypeStringWithDot covers the "strip package prefix" branch in
// typeName when types.TypeString returns a qualified name like "pkg.TypeName".
func TestTypeName_TypeStringWithDot(t *testing.T) {
	// Build a fake "time" package and a slice of time.Time.
	// TypeString([]time.Time, nil) returns "[]time.Time" which contains a dot.
	fakePkg := types.NewPackage("time", "time")
	obj := types.NewTypeName(0, fakePkg, "Time", nil)
	namedTime := types.NewNamed(obj, types.NewStruct(nil, nil), nil)

	// Slice of time.Time — unnamed, so the switch falls through to TypeString.
	sl := types.NewSlice(namedTime)
	got := typeName(sl)
	// TypeString = "[]time.Time"; after stripping at last dot → "Time".
	if got != "Time" {
		t.Errorf("typeName([]time.Time) = %q, want %q", got, "Time")
	}
}
