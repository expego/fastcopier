package fastcopier

import (
	"fmt"
	"reflect"
)

// ── Struct → Map ──────────────────────────────────────────────────────────────

// structToMapPlan copies struct → map[string]V.
type structToMapPlan struct {
	fields []Plan
}

func (p *structToMapPlan) Copy(dst, src reflect.Value, ctx *Context) error {
	if dst.IsNil() {
		dst.Set(reflect.MakeMapWithSize(dst.Type(), len(p.fields)))
	}
	for _, fp := range p.fields {
		if err := fp.Copy(dst, src, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *structToMapPlan) init(ctx *Context, dstType, srcType reflect.Type) error {
	mapKeyType, mapValType := dstType.Key(), dstType.Elem()
	needConvert := false
	switch {
	case strType.AssignableTo(mapKeyType):
	case strType.ConvertibleTo(mapKeyType):
		needConvert = true
	default:
		if ctx.IgnoreNonCopyableTypes {
			return nil
		}
		return fmt.Errorf("%w: copying struct '%v' to 'map[%v]%v' requires string key",
			ErrTypeNonCopyable, srcType, mapKeyType, mapValType)
	}

	srcDirectKeys, mapSrcDirect, srcInhKeys, mapSrcInherited := parseStructFields(srcType, ctx.effectiveTagName())
	p.fields = make([]Plan, 0, len(srcDirectKeys)+len(srcInhKeys))

	for _, key := range append(srcDirectKeys, srcInhKeys...) {
		sm := mapSrcDirect[key]
		if sm == nil {
			sm = mapSrcInherited[key]
		}
		if sm == nil || sm.ignored || sm.done || sm.field.Anonymous {
			continue
		}

		fp, err := p.buildFieldPlan(ctx, mapKeyType, mapValType, srcType, sm, needConvert)
		if err != nil {
			return err
		}
		p.fields = append(p.fields, fp)
		sm.markDone()
	}
	return nil
}

func (p *structToMapPlan) buildFieldPlan(
	ctx *Context,
	mapKeyType, mapValType, srcStructType reflect.Type,
	sm *fieldMeta,
	needConvert bool,
) (Plan, error) {
	sf := sm.field

	mapKey := reflect.ValueOf(sm.key)
	if needConvert {
		mapKey = mapKey.Convert(mapKeyType)
	}

	if scalarKinds&(1<<sf.Type.Kind()) > 0 {
		if sf.Type == mapValType {
			return makeField2MapPlan(sm, mapKey, nil), nil
		}
		if sf.Type.ConvertibleTo(mapValType) {
			return makeField2MapPlan(sm, mapKey, &mapEntryPlan{dstType: mapValType, elem: defaultConvertPlan}), nil
		}
	}

	cp, err := resolvePlan(ctx, mapValType, sf.Type)
	if err != nil {
		if !sm.required && !sf.IsExported() {
			return defaultSkipPlan, nil
		}
		return nil, err
	}
	if ctx.IgnoreNonCopyableTypes && sm.required {
		if _, isSkip := cp.(skipPlan); isSkip {
			return nil, fmt.Errorf("%w: struct field '%v[%s]' requires copying",
				ErrFieldRequireCopying, srcStructType, sm.field.Name)
		}
	}
	return makeField2MapPlan(sm, mapKey, &mapEntryPlan{dstType: mapValType, elem: cp}), nil
}

func makeField2MapPlan(sm *fieldMeta, key reflect.Value, ep *mapEntryPlan) Plan {
	return &field2MapPlan{
		key:         key,
		entry:       ep,
		srcIndex:    sm.index,
		required:    sm.required,
		skipOnError: !sm.field.IsExported(),
	}
}

// field2MapPlan copies a struct field into a map entry.
type field2MapPlan struct {
	key         reflect.Value
	entry       *mapEntryPlan
	srcIndex    []int
	required    bool // true when the field is tagged required
	skipOnError bool // true when copy errors should be silently skipped (unexported fields)
}

func (p *field2MapPlan) Copy(dst, src reflect.Value, ctx *Context) (err error) {
	if len(p.srcIndex) == 1 {
		src = src.Field(p.srcIndex[0])
	} else {
		src, err = src.FieldByIndexErr(p.srcIndex)
		if err != nil {
			return nil //nolint:nilerr
		}
	}

	// SkipZeroFields: skip this field if the source value is the zero value.
	if ctx.SkipZeroFields && src.IsZero() {
		return nil
	}

	if p.entry != nil {
		if src, err = p.entry.copy(src, ctx); err != nil {
			if p.required || !p.skipOnError {
				return err
			}
			return nil
		}
	}
	dst.SetMapIndex(p.key, src)
	return nil
}
