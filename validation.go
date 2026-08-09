package api

import (
	"fmt"
	"log/slog"
	"reflect"
)

// validateTarget runs the configured validator against a decoded response body.
//
// Only structs (and slices of structs) carry validate tags, so targets such as
// map[string]any or the []byte of ByteDecodeStrategy are skipped rather than
// rejected. Passing them to validator.Struct would fail with an
// InvalidValidationError, which says nothing about the response itself.
func (c *Client) validateTarget(target any) error {
	if c.validator == nil || target == nil {
		return nil
	}

	v := deref(reflect.ValueOf(target))

	switch v.Kind() {
	case reflect.Struct:
		return c.validator.StructCtx(c.ctx, v.Interface())

	case reflect.Slice, reflect.Array:
		if !validatableElem(v.Type().Elem()) {
			c.logSkippedValidation(v.Type())
			return nil
		}

		for i := range v.Len() {
			elem := deref(v.Index(i))
			if elem.Kind() != reflect.Struct {
				// Untyped element of an []any that happens not to be a struct.
				continue
			}
			if err := c.validator.StructCtx(c.ctx, elem.Interface()); err != nil {
				return fmt.Errorf("index %d: %w", i, err)
			}
		}
		return nil

	default:
		c.logSkippedValidation(v.Type())
		return nil
	}
}

// deref unwraps pointers and interfaces until it reaches a concrete value.
// A nil along the way yields an invalid Value, whose Kind is reflect.Invalid.
func deref(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

// validatableElem reports whether a slice element type can hold a struct.
// It keeps []byte and friends from being walked element by element.
func validatableElem(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct || t.Kind() == reflect.Interface
}

func (c *Client) logSkippedValidation(t reflect.Type) {
	if t == nil {
		return
	}
	c.logger.DebugContext(c.ctx, "skipping response validation for non-struct target",
		slog.String("type", t.String()),
	)
}
