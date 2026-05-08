package fastcopier

import (
	"reflect"
	"strings"
)

var (
	errType   = reflect.TypeOf((*error)(nil)).Elem()
	ifaceType = reflect.TypeOf((*any)(nil)).Elem()
	strType   = reflect.TypeOf((*string)(nil)).Elem()
)

// fieldMeta holds copying configuration for a single struct field.
type fieldMeta struct {
	field     *reflect.StructField
	key       string // matching name (tag rename or field name)
	ignored   bool
	required  bool
	nilOnZero bool
	renamed   bool // true when key came from an explicit tag rename

	done         bool
	index        []int
	nestedFields []*fieldMeta
}

// markDone marks this field and all promoted nested fields as processed.
func (m *fieldMeta) markDone() {
	m.done = true
	for _, f := range m.nestedFields {
		f.markDone()
	}
}

// slimFieldMeta is a lightweight field descriptor used for map↔struct operations.
type slimFieldMeta struct {
	fieldType  reflect.Type
	key        string
	required   bool
	nilOnZero  bool
	index      []int
	cachedPlan Plan // pre-resolved at init time when map value type is known
}

// applyTag parses the struct tag on m and fills in its configuration.
func applyTag(m *fieldMeta, tagName string) {
	tagValue, ok := m.field.Tag.Lookup(tagName)
	m.key = m.field.Name
	if !ok {
		return
	}

	parts := strings.Split(tagValue, ",")
	switch {
	case parts[0] == "-":
		m.ignored = true
	case parts[0] != "":
		m.key = parts[0]
		m.renamed = true
	}

	for _, opt := range parts[1:] {
		switch opt {
		case "required":
			if !m.ignored {
				m.required = true
			}
		case "nilonzero":
			k := m.field.Type.Kind()
			if k == reflect.Pointer || k == reflect.Interface || k == reflect.Slice || k == reflect.Map {
				m.nilOnZero = true
			}
		}
	}
}

// parseStructFields returns direct and promoted (embedded) fields of typ, keyed by their matching name.
func parseStructFields(typ reflect.Type, tagName string) (
	directKeys []string,
	direct map[string]*fieldMeta,
	inheritedKeys []string,
	inherited map[string]*fieldMeta,
) {
	n := typ.NumField()
	directKeys = make([]string, 0, n)
	direct = make(map[string]*fieldMeta, n)
	inheritedKeys = make([]string, 0, n)
	inherited = make(map[string]*fieldMeta, n)

	for i := 0; i < n; i++ {
		sf := typ.Field(i)
		m := &fieldMeta{field: &sf, index: []int{i}}
		applyTag(m, tagName)
		if m.ignored {
			continue
		}
		if existing, dup := direct[m.key]; dup {
			if !existing.renamed && m.renamed {
				direct[m.key] = m
			}
		} else {
			directKeys = append(directKeys, m.key)
			direct[m.key] = m
		}

		if sf.Anonymous {
			for key, detail := range parseNestedFields(sf.Type, m.index, tagName) {
				inheritedKeys = append(inheritedKeys, key)
				inherited[key] = detail
				m.nestedFields = append(m.nestedFields, detail)
			}
		}
	}
	return
}

// parseNestedFields recursively collects promoted fields from an embedded struct.
// If two embedded structs promote a field with the same name, the field is ambiguous
// (per Go spec) and is excluded from the result — matching Go's own field resolution.
func parseNestedFields(typ reflect.Type, index []int, tagName string) map[string]*fieldMeta {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil
	}
	n := typ.NumField()
	result := make(map[string]*fieldMeta, n)
	// ambiguous tracks keys that appeared more than once (collision → skip).
	ambiguous := make(map[string]bool)

	for i := 0; i < n; i++ {
		sf := typ.Field(i)
		idx := make([]int, len(index)+1)
		copy(idx, index)
		idx[len(index)] = i
		m := &fieldMeta{field: &sf, index: idx}
		applyTag(m, tagName)
		if m.ignored {
			continue
		}
		if ambiguous[m.key] {
			continue
		}
		if _, exists := result[m.key]; exists {
			// Collision: mark ambiguous and remove the previously stored entry.
			ambiguous[m.key] = true
			delete(result, m.key)
			continue
		}
		result[m.key] = m
		if sf.Anonymous {
			for key, detail := range parseNestedFields(sf.Type, m.index, tagName) {
				if ambiguous[key] {
					continue
				}
				if _, exists := result[key]; exists {
					ambiguous[key] = true
					delete(result, key)
					continue
				}
				result[key] = detail
				m.nestedFields = append(m.nestedFields, detail)
			}
		}
	}
	return result
}

// fieldByIndexInit traverses a nested field path, initialising nil pointers along the way.
func fieldByIndexInit(v reflect.Value, index []int) reflect.Value {
	for _, idx := range index {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(idx)
	}
	return v
}

// fieldSetZero sets a nested field to its zero value, ignoring nil-pointer traversal errors.
func fieldSetZero(v reflect.Value, index []int) {
	f, err := v.FieldByIndexErr(index)
	if err == nil && f.IsValid() {
		f.Set(reflect.Zero(f.Type()))
	}
}

// nilOnZeroValue sets a nillable value to nil when its inner value is zero/empty.
func nilOnZeroValue(val reflect.Value) {
	inner := val
	for {
		switch inner.Kind() { //nolint:exhaustive
		case reflect.Pointer, reflect.Interface:
			inner = inner.Elem()
			if !inner.IsValid() || inner.IsZero() {
				val.Set(reflect.Zero(val.Type()))
				return
			}
		case reflect.Slice, reflect.Map:
			if inner.Len() == 0 {
				val.Set(reflect.Zero(val.Type()))
			}
			return
		default:
			return
		}
	}
}
