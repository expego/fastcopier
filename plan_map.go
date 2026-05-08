package fastcopier

import "reflect"

// mapPlan copies map → map.
type mapPlan struct {
	keyEntry *mapEntryPlan
	valEntry *mapEntryPlan
}

func (p *mapPlan) Copy(dst, src reflect.Value, ctx *Context) error {
	if src.IsNil() {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	if dst.IsNil() {
		dst.Set(reflect.MakeMapWithSize(dst.Type(), src.Len()))
	}
	var err error
	iter := src.MapRange()
	for iter.Next() {
		k, v := iter.Key(), iter.Value()
		if p.keyEntry != nil {
			if k, err = p.keyEntry.copy(k, ctx); err != nil {
				return err
			}
		}
		if p.valEntry != nil {
			if v, err = p.valEntry.copy(v, ctx); err != nil {
				return err
			}
		}
		dst.SetMapIndex(k, v)
	}
	return nil
}

func (p *mapPlan) init(ctx *Context, dstType, srcType reflect.Type) error {
	srcKey, srcVal := srcType.Key(), srcType.Elem()
	dstKey, dstVal := dstType.Key(), dstType.Elem()

	needKey, needVal := true, true

	if scalarKinds&(1<<srcKey.Kind()) > 0 {
		if srcKey == dstKey {
			needKey = false
		} else if srcKey.ConvertibleTo(dstKey) {
			p.keyEntry = &mapEntryPlan{dstType: dstKey, elem: defaultConvertPlan}
			needKey = false
		}
	}
	if scalarKinds&(1<<srcVal.Kind()) > 0 {
		if srcVal == dstVal {
			needVal = false
		} else if srcVal.ConvertibleTo(dstVal) {
			p.valEntry = &mapEntryPlan{dstType: dstVal, elem: defaultConvertPlan}
			needVal = false
		}
	}

	if needKey {
		cp, err := resolvePlan(ctx, dstKey, srcKey)
		if err != nil {
			return err
		}
		p.keyEntry = &mapEntryPlan{dstType: dstKey, elem: cp}
	}
	if needVal {
		cp, err := resolvePlan(ctx, dstVal, srcVal)
		if err != nil {
			return err
		}
		p.valEntry = &mapEntryPlan{dstType: dstVal, elem: cp}
	}
	return nil
}

// mapEntryPlan copies a map key or value into a freshly allocated reflect.Value.
type mapEntryPlan struct {
	dstType reflect.Type
	elem    Plan
}

func (e *mapEntryPlan) copy(src reflect.Value, ctx *Context) (reflect.Value, error) {
	dst := reflect.New(e.dstType).Elem()
	return dst, e.elem.Copy(dst, src, ctx)
}
