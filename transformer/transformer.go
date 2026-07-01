// Package transformer provides struct-field transformation for REST requests using
// go-playground/mold. It is the conform/sanitize counterpart to the validator package
// and exposes mold's two faculties:
//
//   - modifiers (`mod:` tags) mutate fields in place — trim, lcase, default, snake, … —
//     and are meant to run before validation so dirty input is cleaned first.
//   - scrubbers (`scrub:` tags) redact sensitive fields — emails, name, … — so a request
//     struct can be logged safely.
//
// The canonical request flow is decode → Transform (conform) → validate. Scrubbing mutates
// in place, so use ScrubbedCopy when the original value must be preserved (e.g. for logging
// a request that the handler still needs).
//
// Basic usage:
//
//	type User struct {
//	    Name  string `json:"name"  mod:"trim"        validate:"required"  scrub:"name"`
//	    Email string `json:"email" mod:"trim,lcase" validate:"required,email" scrub:"emails"`
//	}
//
//	t := transformer.DefaultRestTransformer()
//	_ = t.Transform(ctx, &user) // conform: "  A@B.com " -> "a@b.com"
//	// ... validate, handle ...
//	safe, _ := transformer.ScrubbedCopy(ctx, t, &user) // redacted copy for logging
//
// Custom transformations can be registered via RegisterModifier / RegisterScrubber, or by
// reaching the underlying mold transformers in the Modifiers / Scrubbers fields.
package transformer

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/go-playground/mold/v4"
	"github.com/go-playground/mold/v4/modifiers"
	"github.com/go-playground/mold/v4/scrubbers"
)

var ErrNotTransformable = errors.New("transformer: value is not transformable")

type Transformer struct {
	Modifiers *mold.Transformer // applies `mod:` tags (conform before validation)
	Scrubbers *mold.Transformer // applies `scrub:` tags (redact PII for logging)
}

func DefaultRestTransformer() *Transformer {
	return &Transformer{
		Modifiers: modifiers.New(),
		Scrubbers: scrubbers.New(),
	}
}

func (t *Transformer) Transform(ctx context.Context, i any) error {
	return t.Modifiers.Struct(ctx, i)
}

func (t *Transformer) Scrub(ctx context.Context, i any) error {
	return t.Scrubbers.Struct(ctx, i)
}

func (t *Transformer) RegisterModifier(tag string, fn mold.Func) {
	t.Modifiers.Register(tag, fn)
}

func (t *Transformer) RegisterScrubber(tag string, fn mold.Func) {
	t.Scrubbers.Register(tag, fn)
}

func ScrubbedCopy[T any](ctx context.Context, tfm *Transformer, v *T) (*T, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var clone T
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}

	if err := tfm.Scrub(ctx, &clone); err != nil {
		return nil, err
	}

	return &clone, nil
}

func ScrubbedCopyValue(ctx context.Context, tfm *Transformer, v any) (any, error) {
	if !IsTransformable(v) {
		return nil, ErrNotTransformable
	}

	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	clone := reflect.New(reflect.TypeOf(v).Elem()).Interface()
	if err := json.Unmarshal(data, clone); err != nil {
		return nil, err
	}

	if err := tfm.Scrub(ctx, clone); err != nil {
		return nil, err
	}

	return clone, nil
}

func IsTransformable(i any) bool {
	val := reflect.ValueOf(i)
	if val.Kind() != reflect.Pointer || val.IsNil() {
		return false
	}

	return val.Elem().Kind() == reflect.Struct
}
