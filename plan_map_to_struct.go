package fastcopier

import (
	"fmt"
	"reflect"
)

// ── Map → Struct ──────────────────────────────────────────────────────────────

// mapToStructPlan copies map[string]V → struct.
type mapToStructPlan struct {
	dstFields     map[string]*slimFieldMeta
	requiredCount int
}

func (p *mapToStructPlan) Copy(dstStruct, srcMap reflect.Value, ctx *Context) error {
	if !srcMap.IsValid() || srcMap.IsNil() {
		return nil
	}

	dstStructType := dstStruct.Type()
	var copied map[string]struct{}
	if p.requiredCount > 0 {
		copied = make(map[string]struct{}, p.requiredCount)
	}

	iter := srcMap.MapRange()
	var err error
	for iter.Next() {
		keyStr := iter.Key().String()
		srcVal := iter.Value()

		dm := p.dstFields[keyStr]
		if dm == nil {
			continue
		}

		ep := dm.cachedPlan
		if ep == nil {
			var buildErr error
			ep, buildErr = p.buildEntryPlan(ctx, dstStructType, srcVal.Type(), dm)
			if buildErr != nil {
				return buildErr
			}
		}
		if err = ep.Copy(dstStruct, srcVal, ctx); err != nil {
			return err
		}
		if dm.required {
			copied[dm.key] = struct{}{}
		}
	}

	if p.requiredCount > 0 {
		for _, dm := range p.dstFields {
			if dm.required {
				if _, ok := copied[dm.key]; !ok {
					return fmt.Errorf("%w: struct field '%v[%s]' requires copying",
						ErrFieldRequireCopying, dstStructType, dm.key)
				}
			}
		}
	}
	return nil
}

func (p *mapToStructPlan) init(ctx *Context, dstType, srcType reflect.Type) error {
	if srcType.Key().Kind() != reflect.String {
		if ctx.IgnoreNonCopyableTypes {
			return nil
		}
		return fmt.Errorf("%w: copying from 'map[%v]%v' to struct '%v' requires string key",
			ErrTypeNonCopyable, srcType.Key(), srcType.Elem(), dstType)
	}

	dkKeys, mapDkDirect, dkInhKeys, mapDkInherited := parseStructFields(dstType, ctx.effectiveTagName())
	p.dstFields = make(map[string]*slimFieldMeta, len(dkKeys)+len(dkInhKeys))

	for _, key := range append(dkKeys, dkInhKeys...) {
		dm := mapDkDirect[key]
		if dm == nil || dm.field.Anonymous {
			dm = mapDkInherited[key]
		}
		if dm == nil || dm.ignored || dm.done || dm.field.Anonymous {
			continue
		}
		p.dstFields[dm.key] = &slimFieldMeta{
			key:       dm.key,
			fieldType: dm.field.Type,
			required:  dm.required,
			nilOnZero: dm.nilOnZero,
			index:     dm.index,
		}
		if dm.required {
			p.requiredCount++
		}
	}

	// Pre-resolve entry plans using the map's static value type.
	srcValType := srcType.Elem()
	for key, dm := range p.dstFields {
		cp, err := p.buildEntryPlan(ctx, dstType, srcValType, dm)
		if err == nil {
			p.dstFields[key].cachedPlan = cp
		}
	}
	return nil
}

func (p *mapToStructPlan) buildEntryPlan(ctx *Context, dstStructType, srcValType reflect.Type, dm *slimFieldMeta) (Plan, error) {
	if scalarKinds&(1<<srcValType.Kind()) > 0 {
		if srcValType == dm.fieldType {
			return makeVal2FieldPlan(dm, nil), nil
		}
		if srcValType.ConvertibleTo(dm.fieldType) {
			return makeVal2FieldPlan(dm, defaultConvertPlan), nil
		}
	}

	cp, err := resolvePlan(ctx, dm.fieldType, srcValType)
	if err != nil {
		if !dm.required {
			return defaultSkipPlan, nil
		}
		return nil, err
	}
	if ctx.IgnoreNonCopyableTypes && dm.required {
		if _, isSkip := cp.(skipPlan); isSkip {
			return nil, fmt.Errorf("%w: struct field '%v[%s]' requires copying",
				ErrFieldRequireCopying, dstStructType, dm.key)
		}
	}
	return makeVal2FieldPlan(dm, cp), nil
}

func makeVal2FieldPlan(dm *slimFieldMeta, cp Plan) Plan {
	return &val2FieldPlan{
		elem:         cp,
		dstIndex:     dm.index,
		dstNilOnZero: dm.nilOnZero,
		required:     dm.required,
	}
}

// val2FieldPlan copies a map value (src) into a struct field (dst).
type val2FieldPlan struct {
	elem         Plan
	dstIndex     []int
	dstNilOnZero bool
	required     bool
}

func (p *val2FieldPlan) Copy(dst, src reflect.Value, ctx *Context) (err error) {
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
			if p.required {
				return err
			}
			return nil
		}
	} else {
		dst.Set(src)
	}

	if p.dstNilOnZero {
		nilOnZeroValue(dst)
	}
	return nil
}
