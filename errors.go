package fastcopier

import (
	"errors"
	"fmt"
	"reflect"
)

// Sentinel errors returned by Copy.
var (
	// ErrTypeInvalid is returned when the type of an input variable does not meet requirements.
	ErrTypeInvalid = errors.New("invalid type")
	// ErrTypeNonCopyable is returned when copying between the given types is not supported.
	ErrTypeNonCopyable = errors.New("type is not copyable")
	// ErrValueInvalid is returned when an input value does not meet requirements.
	ErrValueInvalid = errors.New("invalid value")
	// ErrFieldRequireCopying is returned when a required field has no matching counterpart.
	ErrFieldRequireCopying = errors.New("field requires copying")
	// ErrCircularReference is returned when a pointer cycle is detected during copying.
	ErrCircularReference = errors.New("circular reference detected")
)

// CopyError is a structured error that carries field-level context about a copy failure.
// It wraps one of the sentinel errors (ErrTypeNonCopyable, ErrTypeInvalid, etc.) so that
// errors.Is continues to work, while giving callers precise diagnostic information.
//
// Example:
//
//	err := fastcopier.Copy(&dst, &src)
//	var ce *fastcopier.CopyError
//	if errors.As(err, &ce) {
//	    fmt.Printf("failed copying %s → %s (field %s): %v\n",
//	        ce.SrcType, ce.DstType, ce.SrcField, ce.Err)
//	}
type CopyError struct {
	// SrcType is the fully-qualified type name of the source value involved in the failure.
	SrcType string
	// DstType is the fully-qualified type name of the destination value involved in the failure.
	DstType string
	// SrcField is the source struct field name, or empty for top-level errors.
	SrcField string
	// DstField is the destination struct field name, or empty for top-level errors.
	DstField string
	// Err is the underlying sentinel error (e.g. ErrTypeNonCopyable).
	Err error
}

func (e *CopyError) Error() string {
	if e.SrcField != "" || e.DstField != "" {
		field := e.SrcField
		if field == "" {
			field = e.DstField
		}
		return fmt.Sprintf("%v: %v -> %v (field %q)", e.Err, e.SrcType, e.DstType, field)
	}
	return fmt.Sprintf("%v: %v -> %v", e.Err, e.SrcType, e.DstType)
}

// Unwrap returns the underlying sentinel error so errors.Is works transparently.
func (e *CopyError) Unwrap() error { return e.Err }

// newCopyError constructs a CopyError for a type-level failure (no field context).
func newCopyError(err error, srcType, dstType reflect.Type) *CopyError {
	return &CopyError{Err: err, SrcType: srcType.String(), DstType: dstType.String()}
}

// newFieldCopyError constructs a CopyError with field-level context.
func newFieldCopyError(err error, srcType, dstType reflect.Type, srcField, dstField string) *CopyError {
	return &CopyError{Err: err, SrcType: srcType.String(), DstType: dstType.String(), SrcField: srcField, DstField: dstField}
}
