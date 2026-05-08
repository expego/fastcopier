package fastcopier

import (
	"fmt"
	"reflect"
	"strings"
)

// FieldMapping describes how a single field will be copied from source to destination.
type FieldMapping struct {
	// SrcField is the source struct field name (after tag resolution).
	SrcField string
	// DstField is the destination struct field name (after tag resolution).
	DstField string
	// SrcType is the Go type of the source field (e.g. "int32", "[]string").
	SrcType string
	// DstType is the Go type of the destination field.
	DstType string
	// Action describes what the copier will do: "assign", "convert", "deep-copy", or "skip".
	Action string
}

// InspectPlan is a human-readable description of what Copy will do for a given
// (dst, src) type pair. It lists every field that will be copied, how it will be
// copied, and which source fields have no matching destination (and will be silently
// skipped).
//
// Use Inspect at program startup to audit field mappings and catch mismatches early.
// This is especially useful for AI coding agents that need to reason about the
// mapping before executing it.
type InspectPlan struct {
	// SrcType is the fully-qualified type name of the source.
	SrcType string
	// DstType is the fully-qualified type name of the destination.
	DstType string
	// Fields lists every field that will be actively copied.
	Fields []FieldMapping
	// Skipped lists source field names that have no matching destination field.
	Skipped []string
}

// String returns a compact, human-readable summary of the plan.
func (p *InspectPlan) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Copy %s → %s\n", p.SrcType, p.DstType)
	for _, f := range p.Fields {
		if f.SrcType == f.DstType {
			fmt.Fprintf(&b, "  %-20s → %-20s  [%s]  %s\n", f.SrcField, f.DstField, f.SrcType, f.Action)
		} else {
			fmt.Fprintf(&b, "  %-20s → %-20s  %s → %s  [%s]\n", f.SrcField, f.DstField, f.SrcType, f.DstType, f.Action)
		}
	}
	if len(p.Skipped) > 0 {
		fmt.Fprintf(&b, "  skipped (no dst match): %v\n", p.Skipped)
	}
	return b.String()
}

// Inspect builds and returns the copy plan for the given (dst, src) pair without
// executing any copy. dst must be a non-nil pointer; src may be a value or pointer.
//
// The returned InspectPlan describes every field mapping, the action that will be
// taken (assign / convert / deep-copy / skip), and which source fields have no
// matching destination.
//
// Inspect is safe to call concurrently and uses the global plan cache, so repeated
// calls for the same type pair are cheap.
//
// Example:
//
//	plan, err := fastcopier.Inspect(&UserDTO{}, &UserEntity{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Print(plan)
func Inspect(dst, src any, options ...Option) (*InspectPlan, error) {
	if dst == nil || src == nil {
		return nil, fmt.Errorf("%w: source and destination must be non-nil", ErrValueInvalid)
	}

	dstVal := reflect.ValueOf(dst)
	srcVal := reflect.ValueOf(src)
	dstType := dstVal.Type()

	if dstType.Kind() != reflect.Pointer {
		return nil, fmt.Errorf("%w: destination must be a pointer", ErrTypeInvalid)
	}
	dstVal = dstVal.Elem()
	dstType = dstType.Elem()

	if !dstVal.IsValid() {
		return nil, fmt.Errorf("%w: destination must be non-nil", ErrValueInvalid)
	}
	if srcVal.Kind() == reflect.Pointer {
		if srcVal.IsNil() {
			return nil, fmt.Errorf("%w: source must be non-nil", ErrValueInvalid)
		}
		srcVal = srcVal.Elem()
	}

	ctx := defaultCtx()
	for _, opt := range options {
		opt(ctx)
	}
	ctx.prepare()

	plan, err := resolvePlan(ctx, dstType, srcVal.Type())
	ctx.reset()
	ctxPool.Put(ctx)
	if err != nil {
		return nil, err
	}

	ip := &InspectPlan{
		SrcType: srcVal.Type().String(),
		DstType: dstType.String(),
	}
	inspectPlan(plan, ip, srcVal.Type(), dstType)
	return ip, nil
}

// inspectPlan walks a Plan tree and populates ip.
func inspectPlan(plan Plan, ip *InspectPlan, srcType, dstType reflect.Type) {
	switch p := plan.(type) {
	case *structPlan:
		inspectStructPlan(p, ip, srcType, dstType)
	case assignPlan:
		// top-level assign (e.g. Copy(&string, &string))
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "assign",
		})
	case convertPlan:
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "convert",
		})
	case *slicePlan:
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "deep-copy",
		})
	case *mapPlan:
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "deep-copy",
		})
	case *structToMapPlan:
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "deep-copy",
		})
	case *mapToStructPlan:
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "deep-copy",
		})
	case *ifaceSrcPlan:
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "deep-copy",
		})
	case *ifaceDstPlan:
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "deep-copy",
		})
	case *deferredPlan:
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "deep-copy",
		})
	case skipPlan:
		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: srcType.String(),
			DstField: dstType.String(),
			SrcType:  srcType.String(),
			DstType:  dstType.String(),
			Action:   "skip",
		})
	case *ptrPlan:
		// Unwrap one pointer level and recurse.
		dstElem, srcElem := dstType, srcType
		if dstType.Kind() == reflect.Pointer {
			dstElem = dstType.Elem()
		}
		if srcType.Kind() == reflect.Pointer {
			srcElem = srcType.Elem()
		}
		inspectPlan(p.elem, ip, srcElem, dstElem)
	case *derefPlan:
		srcElem := srcType
		if srcType.Kind() == reflect.Pointer {
			srcElem = srcType.Elem()
		}
		inspectPlan(p.elem, ip, srcElem, dstType)
	case *addrPlan:
		dstElem := dstType
		if dstType.Kind() == reflect.Pointer {
			dstElem = dstType.Elem()
		}
		inspectPlan(p.elem, ip, srcType, dstElem)
	}
}

// inspectStructPlan walks a structPlan's field list and classifies each fieldPlan.
func inspectStructPlan(sp *structPlan, ip *InspectPlan, srcType, dstType reflect.Type) {
	// Build a set of src field names covered by the plan.
	coveredSrcNames := make(map[string]struct{}, len(sp.fields))

	for _, rawPlan := range sp.fields {
		fp, ok := rawPlan.(*fieldPlan)
		if !ok {
			continue
		}

		// Resolve types via index (needed for Action classification).
		srcField := srcType.FieldByIndex(fp.srcIndex)
		dstField := dstType.FieldByIndex(fp.dstIndex)

		srcName := fieldNameByIndex(srcType, fp.srcIndex)
		coveredSrcNames[srcName] = struct{}{}

		action := classifyPlan(fp.elem, srcField.Type, dstField.Type)

		ip.Fields = append(ip.Fields, FieldMapping{
			SrcField: fp.srcKey,
			DstField: fp.dstKey,
			SrcType:  srcField.Type.String(),
			DstType:  dstField.Type.String(),
			Action:   action,
		})
	}

	// Collect skipped src fields: fields with no dst match.
	// Use the maps cached in structPlan at plan-build time to avoid re-parsing.
	srcDirect := sp.srcDirect
	srcInherited := sp.srcInherited
	dstDirect := sp.dstDirect
	dstInherited := sp.dstInherited

	allSrcKeys := make([]string, 0, len(srcDirect)+len(srcInherited))
	for k := range srcDirect {
		allSrcKeys = append(allSrcKeys, k)
	}
	for k := range srcInherited {
		if _, already := srcDirect[k]; !already {
			allSrcKeys = append(allSrcKeys, k)
		}
	}

	for _, key := range allSrcKeys {
		sm := srcDirect[key]
		if sm == nil {
			sm = srcInherited[key]
		}
		if sm == nil || sm.ignored {
			continue
		}
		if _, covered := coveredSrcNames[sm.field.Name]; !covered {
			dm := dstDirect[key]
			if dm == nil {
				dm = dstInherited[key]
			}
			if dm == nil {
				ip.Skipped = append(ip.Skipped, key)
			}
		}
	}
}

// classifyPlan returns a human-readable action string for a field-level Plan.
func classifyPlan(p Plan, srcType, dstType reflect.Type) string {
	if p == nil {
		// nil elem in fieldPlan means a direct Set (same type, flat).
		// Struct fields report "deep-copy" even when implemented as a single Set,
		// because conceptually copying a struct is a deep copy operation.
		if srcType.Kind() == reflect.Struct {
			return "deep-copy"
		}
		return "assign"
	}
	switch p.(type) {
	case assignPlan:
		return "assign"
	case convertPlan:
		return "convert"
	case skipPlan:
		return "skip"
	default:
		// Pointer plans, slice plans, struct plans, map plans — all are deep-copy.
		return "deep-copy"
	}
}

// fieldNameByIndex resolves the struct field name from a field index path.
func fieldNameByIndex(t reflect.Type, index []int) string {
	if len(index) == 0 {
		return ""
	}
	f := t.Field(index[0])
	if len(index) == 1 {
		return f.Name
	}
	// Embedded: return the leaf field name.
	ft := f.Type
	if ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}
	return fieldNameByIndex(ft, index[1:])
}
