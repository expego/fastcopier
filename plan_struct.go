package fastcopier

import (
	"errors"
	"fmt"
	"reflect"
)

// ── Struct → Struct ───────────────────────────────────────────────────────────

// structPlan copies one struct to another by running a list of per-field plans.
//
// The four parsed field maps (srcDirect, srcInherited, dstDirect, dstInherited)
// are stored here rather than re-computed on every Inspect call. They are
// populated once during init and are read-only afterwards, so they are safe to
// read from multiple goroutines without locking.
type structPlan struct {
	tagName      string
	fields       []Plan
	srcDirect    map[string]*fieldMeta
	srcInherited map[string]*fieldMeta
	dstDirect    map[string]*fieldMeta
	dstInherited map[string]*fieldMeta
}

func (p *structPlan) Copy(dst, src reflect.Value, ctx *Context) error {
	for _, fp := range p.fields {
		if err := fp.Copy(dst, src, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *structPlan) init(ctx *Context, dstType, srcType reflect.Type) error {
	p.tagName = ctx.effectiveTagName()
	dstDirectKeys, mapDstDirect, dstInhKeys, mapDstInherited := parseStructFields(dstType, p.tagName)
	srcDirectKeys, mapSrcDirect, srcInhKeys, mapSrcInherited := parseStructFields(srcType, p.tagName)

	// Store parsed field maps for use by Inspect, avoiding a second parse there.
	p.srcDirect = mapSrcDirect
	p.srcInherited = mapSrcInherited
	p.dstDirect = mapDstDirect
	p.dstInherited = mapDstInherited

	p.fields = make([]Plan, 0, len(dstDirectKeys)+len(dstInhKeys))

	for _, key := range append(srcDirectKeys, srcInhKeys...) {
		// Field mask: skip fields not explicitly requested.
		if ctx.fieldMask != nil {
			if _, ok := ctx.fieldMask[key]; !ok {
				continue
			}
		}

		sm := mapSrcDirect[key]
		if sm == nil {
			sm = mapSrcInherited[key]
		}
		if sm == nil || sm.ignored || sm.done {
			continue
		}

		dm := mapDstDirect[key]
		if dm == nil {
			dm = mapDstInherited[key]
		}
		if dm == nil || dm.ignored || dm.done {
			if sm.required {
				return fmt.Errorf("%w: struct field '%v[%s]' requires copying",
					ErrFieldRequireCopying, srcType, sm.field.Name)
			}
			continue
		}

		fp, err := p.buildFieldPlan(ctx, dstType, srcType, dm, sm)
		if err != nil {
			return err
		}
		p.fields = append(p.fields, fp)
		dm.markDone()
		sm.markDone()
	}

	for _, dm := range mapDstDirect {
		if !dm.done && dm.required {
			return fmt.Errorf("%w: struct field '%v[%s]' requires copying",
				ErrFieldRequireCopying, dstType, dm.field.Name)
		}
	}
	for _, dm := range mapDstInherited {
		if !dm.done && dm.required {
			return fmt.Errorf("%w: struct field '%v[%s]' requires copying",
				ErrFieldRequireCopying, dstType, dm.field.Name)
		}
	}
	return nil
}

func (p *structPlan) buildFieldPlan(ctx *Context, dstStructType, srcStructType reflect.Type, dm, sm *fieldMeta) (Plan, error) {
	df, sf := dm.field, sm.field

	// Same type and flat: a direct Set is a correct deep copy, no inner plan needed.
	if sf.Type == df.Type && isFlatType(sf.Type) {
		return makeFieldPlan(dm, sm, nil), nil
	}
	if scalarKinds&(1<<sf.Type.Kind()) > 0 {
		if sf.Type.ConvertibleTo(df.Type) {
			return makeFieldPlan(dm, sm, defaultConvertPlan), nil
		}
	}

	cp, err := resolvePlan(ctx, df.Type, sf.Type)
	if err != nil {
		if !dm.required && !sm.required && !df.IsExported() {
			return defaultSkipPlan, nil
		}
		// Wrap with field-level context, preserving the original error sentinel.
		// Use errors.As to extract the underlying sentinel if it's already a CopyError.
		var ce *CopyError
		if errors.As(err, &ce) {
			return nil, newFieldCopyError(ce.Err, srcStructType, dstStructType, sf.Name, df.Name)
		}
		return nil, newFieldCopyError(err, srcStructType, dstStructType, sf.Name, df.Name)
	}
	if ctx.IgnoreNonCopyableTypes && (sm.required || dm.required) {
		if _, isSkip := cp.(skipPlan); isSkip {
			if dm.required {
				return nil, fmt.Errorf("%w: struct field '%v[%s]' requires copying",
					ErrFieldRequireCopying, dstStructType, dm.field.Name)
			}
			return nil, fmt.Errorf("%w: struct field '%v[%s]' requires copying",
				ErrFieldRequireCopying, srcStructType, sm.field.Name)
		}
	}
	return makeFieldPlan(dm, sm, cp), nil
}

// makeFieldPlan builds a fieldPlan that copies a single field from src to dst.
//
// fieldMeta fields used:
//   - dm.index, sm.index  — reflect index path to the field within its struct
//   - dm.nilOnZero        — whether to nil the dst field when its value is zero
//   - dm.key, sm.key      — tag-resolved names stored for Inspect/audit use
//
// fieldMeta fields intentionally ignored here (consulted earlier in buildFieldPlan):
//   - dm/sm.required   — evaluated in init() and buildFieldPlan() before this call
//   - dm/sm.ignored    — filtered out before makeFieldPlan is ever reached
//   - dm/sm.renamed    — informational only; the resolved key is already in dm/sm.key
//   - dm/sm.done       — managed by init() to prevent duplicate processing
//   - dm/sm.field      — the reflect.StructField is only needed for type resolution
//     and error messages in buildFieldPlan, not at copy time
func makeFieldPlan(dm, sm *fieldMeta, cp Plan) Plan {
	return &fieldPlan{
		elem:         cp,
		dstIndex:     dm.index,
		dstNilOnZero: dm.nilOnZero,
		srcIndex:     sm.index,
		srcKey:       sm.key,
		dstKey:       dm.key,
	}
}

// fieldPlan copies a single src field to a dst field within their parent structs.
type fieldPlan struct {
	elem         Plan
	dstIndex     []int
	dstNilOnZero bool
	srcIndex     []int
	// srcKey and dstKey are the tag-resolved field keys stored at plan-build time,
	// used by Inspect to avoid re-parsing struct tags.
	srcKey string
	dstKey string
}

func (p *fieldPlan) Copy(dst, src reflect.Value, ctx *Context) (err error) {
	if len(p.srcIndex) == 1 {
		src = src.Field(p.srcIndex[0])
	} else {
		src, err = src.FieldByIndexErr(p.srcIndex)
		if err != nil {
			fieldSetZero(dst, p.dstIndex)
			return nil //nolint:nilerr
		}
	}

	// SkipZeroFields: skip this field if the source value is the zero value.
	if ctx.SkipZeroFields && src.IsZero() {
		return nil
	}

	if len(p.dstIndex) == 1 {
		dst = dst.Field(p.dstIndex[0])
	} else {
		dst = fieldByIndexInit(dst, p.dstIndex)
	}

	if p.elem != nil {
		if err = p.elem.Copy(dst, src, ctx); err != nil {
			return err
		}
	} else {
		dst.Set(src)
	}

	if p.dstNilOnZero {
		nilOnZeroValue(dst)
	}
	return nil
}
