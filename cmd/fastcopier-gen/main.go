// fastcopier-gen generates zero-reflection copy functions for Go structs.
//
// Usage (same-type copies):
//
//	//go:generate go run github.com/expego/fastcopier/cmd/fastcopier-gen \
//	    -types=Foo,Bar -out=copy_gen.go
//
// Usage (cross-type copy, matching field names):
//
//	//go:generate go run github.com/expego/fastcopier/cmd/fastcopier-gen \
//	    -src=UserEntity -dst=UserDTO -out=copy_gen.go
//
// The generated file registers copy functions with fastcopier.RegisterCopier in
// its init() so that fastcopier.Copy automatically routes to them.  Users who
// do not want generated code simply skip go generate; the reflection engine
// handles everything.  To exclude a generated file from a specific build, pass
// -tags fastcopier_no_gen.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

func main() {
	typesFlag := flag.String("types", "", "Comma-separated type names for same-type copy generation")
	srcFlag := flag.String("src", "", "Source type name (cross-type copy)")
	dstFlag := flag.String("dst", "", "Destination type name (cross-type copy)")
	outFlag := flag.String("out", "fastcopier_gen.go", "Output file name")
	pkgFlag := flag.String("pkg", ".", "Package directory to process")
	flag.Parse()

	if *typesFlag == "" && (*srcFlag == "" || *dstFlag == "") {
		log.Fatal("fastcopier-gen: provide -types OR both -src and -dst")
	}

	dir, err := filepath.Abs(*pkgFlag)
	if err != nil {
		log.Fatalf("fastcopier-gen: resolving package dir: %v", err)
	}

	pkg, err := loadPackage(dir)
	if err != nil {
		log.Fatalf("fastcopier-gen: loading package: %v", err)
	}

	// Collect type pairs to generate.
	var pairs []typePair
	if *typesFlag != "" {
		for _, name := range splitComma(*typesFlag) {
			t := lookupStruct(pkg.scope, name)
			if t == nil {
				log.Fatalf("fastcopier-gen: type %q not found in package", name)
			}
			pairs = append(pairs, typePair{Dst: t, Src: t, SameType: true})
		}
	} else {
		srcTyp := lookupStruct(pkg.scope, *srcFlag)
		if srcTyp == nil {
			log.Fatalf("fastcopier-gen: src type %q not found in package", *srcFlag)
		}
		dstTyp := lookupStruct(pkg.scope, *dstFlag)
		if dstTyp == nil {
			log.Fatalf("fastcopier-gen: dst type %q not found in package", *dstFlag)
		}
		pairs = append(pairs, typePair{Dst: dstTyp, Src: srcTyp, SameType: false})
	}

	out, err := generate(pkg, pairs)
	if err != nil {
		log.Fatalf("fastcopier-gen: generating code: %v", err)
	}

	outPath := filepath.Join(dir, *outFlag)
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		log.Fatalf("fastcopier-gen: writing %s: %v", outPath, err)
	}
	fmt.Printf("fastcopier-gen: wrote %s\n", outPath)
}

// ── Package loading ───────────────────────────────────────────────────────────

type pkgInfo struct {
	name  string
	path  string // as returned by types.Package.Path()
	scope *types.Scope
}

func loadPackage(dir string) (*pkgInfo, error) {
	// If there is no go.mod in or above dir, fall back to GOPATH mode so that
	// loadPackage works on bare directories (e.g. in tests).
	env := os.Environ()
	if !hasGoMod(dir) {
		env = append(env, "GO111MODULE=off", "GOWORK=off")
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:  dir,
		Env:  env,
		// Exclude test files and previously generated files via build tags / patterns.
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("loading package in %s: %w", dir, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found in %s", dir)
	}

	// Pick the first non-test package.
	var pkg *packages.Package
	for _, p := range pkgs {
		if !strings.HasSuffix(p.Name, "_test") {
			pkg = p
			break
		}
	}
	if pkg == nil {
		return nil, fmt.Errorf("no non-test package found in %s", dir)
	}

	if pkg.Types == nil || pkg.Types.Name() == "" {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("loading package in %s: %v", dir, pkg.Errors[0])
		}
		return nil, fmt.Errorf("no packages found in %s", dir)
	}

	if len(pkg.Errors) > 0 && pkg.Types == nil {
		return nil, fmt.Errorf("type-checking %s: %v", dir, pkg.Errors[0])
	}

	return &pkgInfo{name: pkg.Types.Name(), path: pkg.Types.Path(), scope: pkg.Types.Scope()}, nil
}

// hasGoMod reports whether dir or any of its parents contains a go.mod file.
func hasGoMod(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func lookupStruct(scope *types.Scope, name string) *types.Named {
	obj := scope.Lookup(name)
	if obj == nil {
		return nil
	}
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return nil
	}
	if _, ok := named.Underlying().(*types.Struct); !ok {
		return nil
	}
	return named
}

// ── Type model ────────────────────────────────────────────────────────────────

type typePair struct {
	Dst      *types.Named
	Src      *types.Named
	SameType bool
}

// fieldPair matches a destination field with its source counterpart.
type fieldPair struct {
	DstField *types.Var
	SrcField *types.Var
}

// matchFields pairs up fields between dst and src by name.
// For same-type pairs every field matches; for cross-type pairs only
// fields whose names match are included.
// Promoted fields from embedded structs are flattened, matching the behaviour
// of the reflection engine (parseStructFields/parseNestedFields).
func matchFields(dst, src *types.Named) []fieldPair {
	srcByName := flattenFields(src)
	dstFields := flattenFieldList(dst)

	var pairs []fieldPair
	for _, df := range dstFields {
		if !df.Exported() {
			continue
		}
		sf, ok := srcByName[df.Name()]
		if !ok {
			continue
		}
		pairs = append(pairs, fieldPair{DstField: df, SrcField: sf})
	}
	return pairs
}

// flattenFields returns a name→field map for all exported fields of named,
// including promoted fields from embedded structs (depth-first, first-wins for
// non-ambiguous names, ambiguous names are excluded — matching Go spec).
func flattenFields(named *types.Named) map[string]*types.Var {
	result := make(map[string]*types.Var)
	ambiguous := make(map[string]bool)
	collectFields(named.Underlying().(*types.Struct), result, ambiguous)
	return result
}

// flattenFieldList returns an ordered slice of all exported fields of named,
// including promoted fields from embedded structs.
func flattenFieldList(named *types.Named) []*types.Var {
	result := make(map[string]*types.Var)
	ambiguous := make(map[string]bool)
	collectFields(named.Underlying().(*types.Struct), result, ambiguous)
	// Return in declaration order by re-iterating.
	var out []*types.Var
	var visit func(s *types.Struct)
	visit = func(s *types.Struct) {
		for i := 0; i < s.NumFields(); i++ {
			f := s.Field(i)
			if f.Anonymous() {
				ft := f.Type()
				if p, ok := ft.(*types.Pointer); ok {
					ft = p.Elem()
				}
				if n, ok := ft.(*types.Named); ok {
					if st, ok := n.Underlying().(*types.Struct); ok {
						visit(st)
						continue
					}
				}
			}
			if !f.Exported() {
				continue
			}
			if ambiguous[f.Name()] {
				continue
			}
			if result[f.Name()] == f {
				out = append(out, f)
			}
		}
	}
	visit(named.Underlying().(*types.Struct))
	return out
}

func collectFields(s *types.Struct, result map[string]*types.Var, ambiguous map[string]bool) {
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if f.Anonymous() {
			ft := f.Type()
			if p, ok := ft.(*types.Pointer); ok {
				ft = p.Elem()
			}
			if n, ok := ft.(*types.Named); ok {
				if st, ok := n.Underlying().(*types.Struct); ok {
					collectFields(st, result, ambiguous)
					continue
				}
			}
		}
		if !f.Exported() {
			continue
		}
		if ambiguous[f.Name()] {
			continue
		}
		if _, exists := result[f.Name()]; exists {
			ambiguous[f.Name()] = true
			delete(result, f.Name())
			continue
		}
		result[f.Name()] = f
	}
}

// ── Type classification ───────────────────────────────────────────────────────

// isFlatGoType mirrors the runtime isFlatType but operates on go/types.Type.
// A type is flat when it contains no heap-allocated fields (no slices, maps,
// pointers, interfaces, channels) — meaning direct assignment is a deep copy.
func isFlatGoType(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		// All basic types (bool, int*, uint*, float*, complex*, string) are flat.
		return true
	case *types.Signature:
		// Function values are reference types but are treated as flat (same as reflect engine).
		return true
	case *types.Array:
		return isFlatGoType(u.Elem())
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if !isFlatGoType(u.Field(i).Type()) {
				return false
			}
		}
		return true
	default:
		// Slice, Map, Pointer, Interface, Chan → not flat.
		return false
	}
}

// typeKind classifies a types.Type for code generation.
type typeKind int

const (
	kindFlat        typeKind = iota // scalars, flat structs — direct assignment
	kindSliceFlat                   // []flatType — make + builtin copy
	kindSliceStruct                 // []struct — make + loop
	kindMapFlat                     // map[scalar]scalar — make + range
	kindMapComplex                  // map with non-flat value — make + range + fallback
	kindPointer                     // *T — nil check + new + recurse
	kindStruct                      // non-flat struct — recurse or fallback
	kindFallback                    // anything else — fastcopier.Copy fallback
)

func classifyType(t types.Type) typeKind {
	if isFlatGoType(t) {
		return kindFlat
	}
	switch u := t.Underlying().(type) {
	case *types.Slice:
		elem := u.Elem()
		if isFlatGoType(elem) {
			return kindSliceFlat
		}
		if _, ok := elem.Underlying().(*types.Struct); ok {
			return kindSliceStruct
		}
		return kindFallback
	case *types.Map:
		if isFlatGoType(u.Key()) && isFlatGoType(u.Elem()) {
			return kindMapFlat
		}
		return kindMapComplex
	case *types.Pointer:
		return kindPointer
	case *types.Struct:
		return kindStruct
	default:
		return kindFallback
	}
}

// ── Code generation ───────────────────────────────────────────────────────────

type genData struct {
	PkgName       string
	HasCopyFallback bool // true when any field uses fastcopier.Copy fallback
	Funcs         []genFunc
}

type genFunc struct {
	FuncName string
	DstType  string
	SrcType  string
	Lines    []string // body lines (without braces)
}

// generator builds the code for a set of type pairs.
type generator struct {
	pkg        *pkgInfo
	knownTypes map[string]bool   // same-type names with generated copy funcs
	knownPairs map[string]string // "SrcName->DstName" → generated func name for cross-type pairs
	needsFC    bool              // set to true when fastcopier fallback is needed
}

// typeStr returns the Go source representation of t, stripping the current
// package qualifier so that types within the same package are unqualified.
func (g *generator) typeStr(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p.Path() == g.pkg.path {
			return "" // same package — no qualifier needed
		}
		return p.Name()
	})
}

func generate(pkg *pkgInfo, pairs []typePair) ([]byte, error) {
	// Build the set of known type names up front so cross-references work.
	known := make(map[string]bool, len(pairs))
	knownPairs := make(map[string]string, len(pairs))
	for _, p := range pairs {
		srcName := p.Src.Obj().Name()
		dstName := p.Dst.Obj().Name()
		known[dstName] = true
		known[srcName] = true
		funcName := "Copy" + srcName + "To" + dstName
		knownPairs[srcName+"->"+dstName] = funcName
	}

	g := &generator{pkg: pkg, knownTypes: known, knownPairs: knownPairs}

	data := genData{
		PkgName:         pkg.name,
		HasCopyFallback: false, // set below after all funcs are built
	}

	for _, pair := range pairs {
		fn, err := g.genFunc(pair)
		if err != nil {
			return nil, err
		}
		data.Funcs = append(data.Funcs, fn)
	}
	data.HasCopyFallback = g.needsFC

	var buf bytes.Buffer
	if err := fileTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return the unformatted source to help debug template issues.
		return buf.Bytes(), fmt.Errorf("formatting generated code: %w\n--- unformatted ---\n%s", err, buf.String())
	}
	return formatted, nil
}

func (g *generator) genFunc(pair typePair) (genFunc, error) {
	dstName := pair.Dst.Obj().Name()
	srcName := pair.Src.Obj().Name()

	fn := genFunc{
		FuncName: "Copy" + srcName + "To" + dstName,
		DstType:  dstName,
		SrcType:  srcName,
	}

	fps := matchFields(pair.Dst, pair.Src)
	for _, fp := range fps {
		lines, err := g.fieldLines(fp)
		if err != nil {
			return genFunc{}, err
		}
		fn.Lines = append(fn.Lines, lines...)
	}
	return fn, nil
}

// fieldLines returns the Go statements that copy fp.SrcField into fp.DstField.
func (g *generator) fieldLines(fp fieldPair) ([]string, error) {
	fname := fp.DstField.Name()
	kind := classifyType(fp.SrcField.Type())

	switch kind {
	case kindFlat:
		// Direct assignment — covers scalars, flat structs, flat arrays.
		return []string{fmt.Sprintf("dst.%s = src.%s", fname, fname)}, nil

	case kindSliceFlat:
		// Same-type scalar slice: reuse capacity when possible, then copy.
		return g.sliceFlatLines(fname, fp.SrcField.Type()), nil

	case kindSliceStruct:
		return g.sliceStructLines(fname, fp.SrcField.Type(), fp.DstField.Type()), nil

	case kindMapFlat:
		return g.mapLines(fname, fp.SrcField.Type(), false), nil

	case kindMapComplex:
		return g.mapLines(fname, fp.SrcField.Type(), true), nil

	case kindPointer:
		return g.pointerLines(fname, fp.SrcField.Type()), nil

	case kindStruct:
		return g.structLines(fname, fp.SrcField.Type()), nil

	default:
		// Unknown / unsupported type — fall back to reflection.
		g.needsFC = true
		return []string{
			fmt.Sprintf("if err := fastcopier.Copy(&dst.%s, &src.%s); err != nil { return err }", fname, fname),
		}, nil
	}
}

func (g *generator) sliceFlatLines(fname string, t types.Type) []string {
	return []string{
		fmt.Sprintf("if src.%s != nil {", fname),
		fmt.Sprintf("  if cap(dst.%s) >= len(src.%s) {", fname, fname),
		fmt.Sprintf("    dst.%s = dst.%s[:len(src.%s)]", fname, fname, fname),
		"  } else {",
		fmt.Sprintf("    dst.%s = make(%s, len(src.%s))", fname, g.typeStr(t), fname),
		"  }",
		fmt.Sprintf("  copy(dst.%s, src.%s)", fname, fname),
		"} else {",
		fmt.Sprintf("  dst.%s = nil", fname),
		"}",
	}
}

func (g *generator) sliceStructLines(fname string, srcType, dstType types.Type) []string {
	srcSl := srcType.Underlying().(*types.Slice)
	dstSl := dstType.Underlying().(*types.Slice)
	srcElemName := typeName(srcSl.Elem())
	dstElemName := typeName(dstSl.Elem())
	var copyLine string
	crossKey := srcElemName + "->" + dstElemName
	if fn, ok := g.knownPairs[crossKey]; ok {
		copyLine = fmt.Sprintf("  if err := %s(&dst.%s[i], &src.%s[i]); err != nil { return err }", fn, fname, fname)
	} else if g.knownTypes[srcElemName] && srcElemName == dstElemName {
		funcName := "Copy" + srcElemName + "To" + srcElemName
		copyLine = fmt.Sprintf("  if err := %s(&dst.%s[i], &src.%s[i]); err != nil { return err }", funcName, fname, fname)
	} else {
		// Fall back to reflection for the element copy.
		g.needsFC = true
		copyLine = fmt.Sprintf("  if err := fastcopier.Copy(&dst.%s[i], &src.%s[i]); err != nil { return err }", fname, fname)
	}
	return []string{
		fmt.Sprintf("if src.%s != nil {", fname),
		fmt.Sprintf("  if cap(dst.%s) >= len(src.%s) {", fname, fname),
		fmt.Sprintf("    dst.%s = dst.%s[:len(src.%s)]", fname, fname, fname),
		"  } else {",
		fmt.Sprintf("    dst.%s = make(%s, len(src.%s))", fname, g.typeStr(dstType), fname),
		"  }",
		fmt.Sprintf("  for i := range src.%s {", fname),
		copyLine,
		"  }",
		"} else {",
		fmt.Sprintf("  dst.%s = nil", fname),
		"}",
	}
}

func (g *generator) mapLines(fname string, t types.Type, complex bool) []string {
	lines := []string{
		fmt.Sprintf("if src.%s != nil {", fname),
		fmt.Sprintf("  dst.%s = make(%s, len(src.%s))", fname, g.typeStr(t), fname),
		fmt.Sprintf("  for k, v := range src.%s {", fname),
	}
	if complex {
		g.needsFC = true
		// For maps with non-flat values, use fastcopier for value copy.
		m := t.Underlying().(*types.Map)
		lines = append(lines,
			fmt.Sprintf("    var dstVal %s", g.typeStr(m.Elem())),
			"    if err := fastcopier.Copy(&dstVal, &v); err != nil { return err }",
			fmt.Sprintf("    dst.%s[k] = dstVal", fname),
		)
	} else {
		lines = append(lines, fmt.Sprintf("    dst.%s[k] = v", fname))
	}
	lines = append(lines,
		"  }",
		"} else {",
		fmt.Sprintf("  dst.%s = nil", fname),
		"}",
	)
	return lines
}

func (g *generator) pointerLines(fname string, t types.Type) []string {
	pt := t.Underlying().(*types.Pointer)
	elemName := typeName(pt.Elem())
	if g.knownTypes[elemName] {
		funcName := "Copy" + elemName + "To" + elemName
		return []string{
			fmt.Sprintf("if src.%s != nil {", fname),
			fmt.Sprintf("  dst.%s = new(%s)", fname, elemName),
			fmt.Sprintf("  if err := %s(dst.%s, src.%s); err != nil { return err }", funcName, fname, fname),
			"} else {",
			fmt.Sprintf("  dst.%s = nil", fname),
			"}",
		}
	}
	// Flat pointer: direct dereference copy.
	if isFlatGoType(pt.Elem()) {
		return []string{
			fmt.Sprintf("if src.%s != nil {", fname),
			fmt.Sprintf("  v := *src.%s", fname),
			fmt.Sprintf("  dst.%s = &v", fname),
			"} else {",
			fmt.Sprintf("  dst.%s = nil", fname),
			"}",
		}
	}
	// Unknown struct pointer — fall back.
	g.needsFC = true
	return []string{
		fmt.Sprintf("if src.%s != nil {", fname),
		fmt.Sprintf("  dst.%s = new(%s)", fname, elemName),
		fmt.Sprintf("  if err := fastcopier.Copy(dst.%s, src.%s); err != nil { return err }", fname, fname),
		"} else {",
		fmt.Sprintf("  dst.%s = nil", fname),
		"}",
	}
}

func (g *generator) structLines(fname string, t types.Type) []string {
	sname := typeName(t)
	if g.knownTypes[sname] {
		funcName := "Copy" + sname + "To" + sname
		return []string{
			fmt.Sprintf("if err := %s(&dst.%s, &src.%s); err != nil { return err }", funcName, fname, fname),
		}
	}
	// Not in the generated set — fall back to reflection.
	g.needsFC = true
	return []string{
		fmt.Sprintf("if err := fastcopier.Copy(&dst.%s, &src.%s); err != nil { return err }", fname, fname),
	}
}

// ── Type name helpers ─────────────────────────────────────────────────────────

// typeName returns just the unqualified base name of a type,
// used when constructing generated function names (e.g. "Copy" + typeName + "To" + typeName).
func typeName(t types.Type) string {
	switch u := t.(type) {
	case *types.Named:
		return u.Obj().Name()
	case *types.Pointer:
		if n, ok := u.Elem().(*types.Named); ok {
			return n.Obj().Name()
		}
	}
	// For unnamed types, fall back to TypeString and strip package prefix.
	s := types.TypeString(t, nil)
	if dot := strings.LastIndex(s, "."); dot >= 0 {
		s = s[dot+1:]
	}
	return s
}

// ── Templates ─────────────────────────────────────────────────────────────────

var fileTmpl = template.Must(template.New("file").Funcs(template.FuncMap{
	"join": strings.Join,
}).Parse(`// Code generated by fastcopier-gen. DO NOT EDIT.

//go:build !fastcopier_no_gen

package {{.PkgName}}

import (
	fastcopier "github.com/expego/fastcopier"
)

func init() {
{{- range .Funcs}}
	fastcopier.RegisterCopier({{.FuncName}})
{{- end}}
}
{{range .Funcs}}
// {{.FuncName}} copies src into dst with zero reflection overhead.
// It is auto-registered with fastcopier.RegisterCopier so that
// fastcopier.Copy(&dst, &src) routes here automatically.
// You may also call it directly for the lowest possible overhead.
func {{.FuncName}}(dst *{{.DstType}}, src *{{.SrcType}}) error {
{{- range .Lines}}
	{{.}}
{{- end}}
	return nil
}
{{end}}`))

// ── Helpers ───────────────────────────────────────────────────────────────────

func splitComma(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
