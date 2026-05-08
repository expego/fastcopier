package benchmarks

import (
	"encoding/json"
	"testing"

	fastcopier "github.com/expego/fastcopier"
	goclone "github.com/huandu/go-clone"
	"github.com/jinzhu/copier"
	mapstructure "github.com/go-viper/mapstructure/v2"
	deepcopy "github.com/tiendc/go-deepcopy"
	"github.com/ulule/deepcopier"
)

// ── Simple Struct (5 primitive fields) ───────────────────────────────────────

func BenchmarkManual_Simple(b *testing.B) {
	src := BenchSimple{Name: "John Doe", Age: 30, Email: "john@example.com", Score: 95.5, Active: true}
	var dst BenchSimple
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst.Name = src.Name
		dst.Age = src.Age
		dst.Email = src.Email
		dst.Score = src.Score
		dst.Active = src.Active
	}
	_ = dst
}

func BenchmarkFastCopier_Simple(b *testing.B) {
	src := BenchSimple{
		Name:   "John Doe",
		Age:    30,
		Email:  "john@example.com",
		Score:  95.5,
		Active: true,
	}
	var dst BenchSimple

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fastcopier.Copy(&dst, &src)
	}
}

func BenchmarkClone_Simple(b *testing.B) {
	src := BenchSimple{Name: "John Doe", Age: 30, Email: "john@example.com", Score: 95.5, Active: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fastcopier.Clone(src)
	}
}

func BenchmarkGoDeepCopy_Simple(b *testing.B) {
	src := BenchSimple{
		Name:   "John Doe",
		Age:    30,
		Email:  "john@example.com",
		Score:  95.5,
		Active: true,
	}
	var dst BenchSimple

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepcopy.Copy(&dst, &src)
	}
}

func BenchmarkJinzhuCopier_Simple(b *testing.B) {
	src := BenchSimple{
		Name:   "John Doe",
		Age:    30,
		Email:  "john@example.com",
		Score:  95.5,
		Active: true,
	}
	var dst BenchSimple

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = copier.Copy(&dst, &src)
	}
}

func BenchmarkDeepcopier_Simple(b *testing.B) {
	src := BenchSimple{
		Name:   "John Doe",
		Age:    30,
		Email:  "john@example.com",
		Score:  95.5,
		Active: true,
	}
	var dst BenchSimple

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepcopier.Copy(&src).To(&dst)
	}
}

func BenchmarkMapstructure_Simple(b *testing.B) {
	src := BenchSimple{
		Name:   "John Doe",
		Age:    30,
		Email:  "john@example.com",
		Score:  95.5,
		Active: true,
	}
	var dst BenchSimple
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mapstructure.Decode(src, &dst)
	}
}

func BenchmarkHuanduGoClone_Simple(b *testing.B) {
	src := BenchSimple{
		Name:   "John Doe",
		Age:    30,
		Email:  "john@example.com",
		Score:  95.5,
		Active: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = goclone.Clone(src).(BenchSimple)
	}
}

func BenchmarkJSONRoundTrip_Simple(b *testing.B) {
	src := BenchSimple{
		Name:   "John Doe",
		Age:    30,
		Email:  "john@example.com",
		Score:  95.5,
		Active: true,
	}
	var dst BenchSimple
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(src)
		_ = json.Unmarshal(data, &dst)
	}
}

// ── Nested Struct (struct + slices) ──────────────────────────────────────────

func BenchmarkManual_Nested(b *testing.B) {
	src := BenchNested{
		ID:      1,
		Profile: BenchSimple{Name: "Jane Doe", Age: 25, Email: "jane@example.com", Score: 88.0, Active: true},
		Tags:    []string{"golang", "performance", "testing"},
		Scores:  []int{1, 2, 3, 4, 5},
	}
	var dst BenchNested
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst.ID = src.ID
		dst.Profile.Name = src.Profile.Name
		dst.Profile.Age = src.Profile.Age
		dst.Profile.Email = src.Profile.Email
		dst.Profile.Score = src.Profile.Score
		dst.Profile.Active = src.Profile.Active
		dst.Tags = make([]string, len(src.Tags))
		copy(dst.Tags, src.Tags)
		dst.Scores = make([]int, len(src.Scores))
		copy(dst.Scores, src.Scores)
	}
	_ = dst
}

func BenchmarkFastCopier_Nested(b *testing.B) {
	src := BenchNested{
		ID: 1,
		Profile: BenchSimple{
			Name:   "Jane Doe",
			Age:    25,
			Email:  "jane@example.com",
			Score:  88.0,
			Active: true,
		},
		Tags:   []string{"golang", "performance", "testing"},
		Scores: []int{1, 2, 3, 4, 5},
	}
	var dst BenchNested

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fastcopier.Copy(&dst, &src)
	}
}

func BenchmarkClone_Nested(b *testing.B) {
	src := BenchNested{
		ID:      1,
		Profile: BenchSimple{Name: "Jane Doe", Age: 25, Email: "jane@example.com", Score: 88.0, Active: true},
		Tags:    []string{"golang", "performance", "testing"},
		Scores:  []int{1, 2, 3, 4, 5},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fastcopier.Clone(src)
	}
}

func BenchmarkGoDeepCopy_Nested(b *testing.B) {
	src := BenchNested{
		ID: 1,
		Profile: BenchSimple{
			Name:   "Jane Doe",
			Age:    25,
			Email:  "jane@example.com",
			Score:  88.0,
			Active: true,
		},
		Tags:   []string{"golang", "performance", "testing"},
		Scores: []int{1, 2, 3, 4, 5},
	}
	var dst BenchNested

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepcopy.Copy(&dst, &src)
	}
}

func BenchmarkJinzhuCopier_Nested(b *testing.B) {
	src := BenchNested{
		ID: 1,
		Profile: BenchSimple{
			Name:   "Jane Doe",
			Age:    25,
			Email:  "jane@example.com",
			Score:  88.0,
			Active: true,
		},
		Tags:   []string{"golang", "performance", "testing"},
		Scores: []int{1, 2, 3, 4, 5},
	}
	var dst BenchNested

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = copier.Copy(&dst, &src)
	}
}

func BenchmarkDeepcopier_Nested(b *testing.B) {
	src := BenchNested{
		ID: 1,
		Profile: BenchSimple{
			Name:   "Jane Doe",
			Age:    25,
			Email:  "jane@example.com",
			Score:  88.0,
			Active: true,
		},
		Tags:   []string{"golang", "performance", "testing"},
		Scores: []int{1, 2, 3, 4, 5},
	}
	var dst BenchNested

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepcopier.Copy(&src).To(&dst)
	}
}

func BenchmarkMapstructure_Nested(b *testing.B) {
	src := BenchNested{
		ID:      1,
		Profile: BenchSimple{Name: "Jane Doe", Age: 25, Email: "jane@example.com", Score: 88.0, Active: true},
		Tags:    []string{"golang", "performance", "testing"},
		Scores:  []int{1, 2, 3, 4, 5},
	}
	var dst BenchNested
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mapstructure.Decode(src, &dst)
	}
}

func BenchmarkHuanduGoClone_Nested(b *testing.B) {
	src := BenchNested{
		ID:      1,
		Profile: BenchSimple{Name: "Jane Doe", Age: 25, Email: "jane@example.com", Score: 88.0, Active: true},
		Tags:    []string{"golang", "performance", "testing"},
		Scores:  []int{1, 2, 3, 4, 5},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = goclone.Clone(src).(BenchNested)
	}
}

func BenchmarkJSONRoundTrip_Nested(b *testing.B) {
	src := BenchNested{
		ID:      1,
		Profile: BenchSimple{Name: "Jane Doe", Age: 25, Email: "jane@example.com", Score: 88.0, Active: true},
		Tags:    []string{"golang", "performance", "testing"},
		Scores:  []int{1, 2, 3, 4, 5},
	}
	var dst BenchNested
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(src)
		_ = json.Unmarshal(data, &dst)
	}
}

// ── Complex Struct (nested + slice of structs + map) ─────────────────────────

func BenchmarkManual_Complex(b *testing.B) {
	src := BenchComplex{
		ID:   100,
		Name: "Complex Test",
		Nested: BenchNested{
			ID:      1,
			Profile: BenchSimple{Name: "Nested Profile", Age: 35, Email: "nested@example.com"},
			Tags:    []string{"tag1", "tag2"},
			Scores:  []int{10, 20, 30},
		},
		Items: []BenchSimple{
			{Name: "Item1", Age: 10},
			{Name: "Item2", Age: 20},
			{Name: "Item3", Age: 30},
		},
		Metadata: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
	}
	var dst BenchComplex
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst.ID = src.ID
		dst.Name = src.Name
		dst.Nested.ID = src.Nested.ID
		dst.Nested.Profile = src.Nested.Profile
		dst.Nested.Tags = make([]string, len(src.Nested.Tags))
		copy(dst.Nested.Tags, src.Nested.Tags)
		dst.Nested.Scores = make([]int, len(src.Nested.Scores))
		copy(dst.Nested.Scores, src.Nested.Scores)
		dst.Items = make([]BenchSimple, len(src.Items))
		copy(dst.Items, src.Items)
		dst.Metadata = make(map[string]string, len(src.Metadata))
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
	}
	_ = dst
}

func BenchmarkFastCopier_Complex(b *testing.B) {
	src := BenchComplex{
		ID:   100,
		Name: "Complex Test",
		Nested: BenchNested{
			ID: 1,
			Profile: BenchSimple{
				Name:  "Nested Profile",
				Age:   35,
				Email: "nested@example.com",
			},
			Tags:   []string{"tag1", "tag2"},
			Scores: []int{10, 20, 30},
		},
		Items: []BenchSimple{
			{Name: "Item1", Age: 10},
			{Name: "Item2", Age: 20},
			{Name: "Item3", Age: 30},
		},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}
	var dst BenchComplex

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fastcopier.Copy(&dst, &src)
	}
}

func BenchmarkClone_Complex(b *testing.B) {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fastcopier.Clone(src)
	}
}

func BenchmarkGoDeepCopy_Complex(b *testing.B) {
	src := BenchComplex{
		ID:   100,
		Name: "Complex Test",
		Nested: BenchNested{
			ID: 1,
			Profile: BenchSimple{
				Name:  "Nested Profile",
				Age:   35,
				Email: "nested@example.com",
			},
			Tags:   []string{"tag1", "tag2"},
			Scores: []int{10, 20, 30},
		},
		Items: []BenchSimple{
			{Name: "Item1", Age: 10},
			{Name: "Item2", Age: 20},
			{Name: "Item3", Age: 30},
		},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}
	var dst BenchComplex

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepcopy.Copy(&dst, &src)
	}
}

func BenchmarkJinzhuCopier_Complex(b *testing.B) {
	src := BenchComplex{
		ID:   100,
		Name: "Complex Test",
		Nested: BenchNested{
			ID: 1,
			Profile: BenchSimple{
				Name:  "Nested Profile",
				Age:   35,
				Email: "nested@example.com",
			},
			Tags:   []string{"tag1", "tag2"},
			Scores: []int{10, 20, 30},
		},
		Items: []BenchSimple{
			{Name: "Item1", Age: 10},
			{Name: "Item2", Age: 20},
			{Name: "Item3", Age: 30},
		},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}
	var dst BenchComplex

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = copier.Copy(&dst, &src)
	}
}

func BenchmarkDeepcopier_Complex(b *testing.B) {
	src := BenchComplex{
		ID:   100,
		Name: "Complex Test",
		Nested: BenchNested{
			ID: 1,
			Profile: BenchSimple{
				Name:  "Nested Profile",
				Age:   35,
				Email: "nested@example.com",
			},
			Tags:   []string{"tag1", "tag2"},
			Scores: []int{10, 20, 30},
		},
		Items: []BenchSimple{
			{Name: "Item1", Age: 10},
			{Name: "Item2", Age: 20},
			{Name: "Item3", Age: 30},
		},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}
	var dst BenchComplex

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepcopier.Copy(&src).To(&dst)
	}
}

func BenchmarkMapstructure_Complex(b *testing.B) {
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
		_ = mapstructure.Decode(src, &dst)
	}
}

func BenchmarkHuanduGoClone_Complex(b *testing.B) {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = goclone.Clone(src).(BenchComplex)
	}
}

func BenchmarkJSONRoundTrip_Complex(b *testing.B) {
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
		data, _ := json.Marshal(src)
		_ = json.Unmarshal(data, &dst)
	}
}

// ── Deep Struct (Organisation with 10 employees, nested pointers, maps) ──────
// Types, makeOrganisation, and manual copy helpers live in bench_types.go.

func BenchmarkManual_Deep(b *testing.B) {
	src := makeOrganisation()
	var dst Organisation
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		visited := make(map[*Employee]*Employee)
		dst.ID = src.ID
		dst.Name = src.Name
		dst.Founded = src.Founded
		dst.HeadOffice = src.HeadOffice
		dst.Metadata = make(map[string]string, len(src.Metadata))
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
		dst.Departments = make([]Department, len(src.Departments))
		for i, d := range src.Departments {
			dst.Departments[i] = Department{
				ID:      d.ID,
				Name:    d.Name,
				Budget:  d.Budget,
				Manager: copyEmployee(d.Manager, visited),
			}
		}
		dst.Employees = make([]Employee, len(src.Employees))
		for i, e := range src.Employees {
			dst.Employees[i] = *copyEmployee(&e, visited)
		}
	}
	_ = dst
}

func BenchmarkFastCopier_Deep(b *testing.B) {
	src := makeOrganisation()
	var dst Organisation
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fastcopier.Copy(&dst, &src)
	}
}

func BenchmarkClone_Deep(b *testing.B) {
	src := makeOrganisation()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fastcopier.Clone(src)
	}
}

// NOTE: BenchmarkGoDeepCopy_Deep is intentionally omitted — go-deepcopy v1.2.1
// stack-overflows on the self-referential Employee.ReportsTo *Employee pointer,
// exposing a missing circular-reference guard in that library.

func BenchmarkJinzhuCopier_Deep(b *testing.B) {
	src := makeOrganisation()
	var dst Organisation
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = copier.Copy(&dst, &src)
	}
}

func BenchmarkDeepcopier_Deep(b *testing.B) {
	src := makeOrganisation()
	var dst Organisation
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepcopier.Copy(&src).To(&dst)
	}
}

// NOTE: BenchmarkMapstructure_Deep is intentionally omitted.
// go-viper/mapstructure v2.5.0 is a map→struct decoder, not a general deep
// copier; it cannot walk the circular Employee.ReportsTo *Employee /
// Employee.Department.Manager *Employee cycle without a stack-overflow.

// NOTE: BenchmarkHuanduGoClone_Deep is intentionally omitted.
// huandu/go-clone v1.7.3 stack-overflows on the same circular-reference cycle.

// NOTE: BenchmarkJSONRoundTrip_Deep is intentionally omitted.
// json.Marshal loops infinitely on the circular Employee.ReportsTo *Employee pointer.
