package fastcopier

import "reflect"

// slicePlan copies a slice or array to a slice or array.
type slicePlan struct {
	elem     Plan
	bulkCopy bool // true when items can be bulk-copied via reflect.Copy
}

func (p *slicePlan) Copy(dst, src reflect.Value, ctx *Context) error {
	srcLen := src.Len()

	if dst.Kind() == reflect.Slice {
		if src.Kind() == reflect.Slice && src.IsNil() {
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}

		if p.bulkCopy {
			if dst.Cap() >= srcLen {
				dst.SetLen(srcLen)
				reflect.Copy(dst, src)
				return nil
			}
			out := reflect.MakeSlice(dst.Type(), srcLen, srcLen)
			reflect.Copy(out, src)
			dst.Set(out)
			return nil
		}

		reused := false
		var out reflect.Value
		if dst.Cap() >= srcLen {
			dst.SetLen(srcLen)
			out = dst
			reused = true
		} else {
			out = reflect.MakeSlice(dst.Type(), srcLen, srcLen)
		}
		for i := 0; i < srcLen; i++ {
			if err := p.elem.Copy(out.Index(i), src.Index(i), ctx); err != nil {
				return err
			}
		}
		if !reused {
			dst.Set(out)
		}
		return nil
	}

	// dst is array: copy up to min(srcLen, dstLen) elements, zero the rest.
	dstLen := dst.Len()
	copyLen := srcLen
	if dstLen < copyLen {
		copyLen = dstLen
	}
	i := 0
	for ; i < copyLen; i++ {
		if err := p.elem.Copy(dst.Index(i), src.Index(i), ctx); err != nil {
			return err
		}
	}
	// Zero remaining dst elements. Hoist the zero value outside the loop since
	// all elements share the same type.
	if i < dstLen {
		zeroVal := reflect.Zero(dst.Type().Elem())
		for ; i < dstLen; i++ {
			dst.Index(i).Set(zeroVal)
		}
	}
	return nil
}

func (p *slicePlan) init(ctx *Context, dstType, srcType reflect.Type) (err error) {
	dstElem, srcElem := dstType.Elem(), srcType.Elem()
	// Bulk-copy path: same element type and flat (no heap-allocated fields).
	// This covers scalars (string, int, float64, …), flat arrays, and flat structs.
	// reflect.Copy is equivalent to a single memcpy of the backing array, which is a
	// correct deep copy when the element type contains no pointers.
	if dstElem == srcElem && isFlatType(srcElem) {
		p.bulkCopy = true
		p.elem = defaultAssignPlan
		return nil
	}
	p.elem, err = resolvePlan(ctx, dstElem, srcElem)
	return
}
