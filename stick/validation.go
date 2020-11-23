package stick

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/256dpi/xo"
	"github.com/asaskevich/govalidator"
)

// Validatable represents a type that can be validated.
type Validatable interface {
	Validate() error
}

// NoValidation can be embedded in a struct to provide a no-op validation method.
type NoValidation struct{}

// Validate will perform no validation.
func (*NoValidation) Validate() error {
	return nil
}

// ValidationError describes a validation error.
type ValidationError struct {
	Reports []ValidationReport
}

// ValidationReport is a single validation error.
type ValidationReport struct {
	Path  []string
	Error error
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	// prepare errors
	var errs []string
	for _, report := range e.Reports {
		errs = append(errs, fmt.Sprintf("%s: %s", strings.Join(report.Path, "."), report.Error.Error()))
	}

	return strings.Join(errs, "; ")
}

// Validator is used to validate an object.
type Validator struct {
	obj     Accessible
	path    []string
	reports []ValidationReport
}

// Validate will validate the provided accessible using the specified validator
// function.
func Validate(obj Accessible, fn func(v *Validator)) error {
	// prepare validator
	val := &Validator{obj: obj}

	// run validator
	fn(val)

	return val.Error()
}

// Nest nest validation under the specified field.
func (v *Validator) Nest(field string, fn func()) {
	// push
	v.path = append(v.path, field)

	// yield
	fn()

	// pop
	v.path = v.path[:len(v.path)-1]
}

// Value will validate the value at the named field using the provided rules.
// If the value is optional it will be skipped if nil or unwrapped if present.
func (v *Validator) Value(name string, optional bool, rules ...Rule) {
	// get value
	value := MustGet(v.obj, name)

	// prepare context
	ctx := RuleContext{
		IValue: value,
		RValue: reflect.ValueOf(value),
	}

	// handle optionals
	if optional {
		// skip if nil
		if ctx.IsNil() {
			return
		}

		// otherwise unwrap pointer once
		ctx.RValue = ctx.RValue.Elem()
		ctx.IValue = ctx.RValue.Interface()
	}

	// execute rules
	for _, rule := range rules {
		err := rule(ctx)
		if err != nil {
			v.Report(name, err)
		}
	}
}

// Items will validate each item of the slice at the named field using the
// provided rules.
func (v *Validator) Items(name string, rules ...Rule) {
	// get slice
	slice := reflect.ValueOf(MustGet(v.obj, name))

	// execute rules for each item
	v.Nest(name, func() {
		for i := 0; i < slice.Len(); i++ {
			// get item
			item := slice.Index(i)

			// prepare context
			ctx := RuleContext{
				IValue: item.Interface(),
				RValue: item,
			}

			// execute rules
			for _, rule := range rules {
				err := rule(ctx)
				if err != nil {
					v.Report(strconv.Itoa(i), err)
				}
			}
		}
	})
}

// Report will report a validation error.
func (v *Validator) Report(name string, err error) {
	// copy path
	path := append([]string{}, v.path...)
	path = append(path, name)

	// add report
	v.reports = append(v.reports, ValidationReport{
		Path:  path,
		Error: err,
	})
}

// Error will return the accumulated error or nil of no errors have yet been
// reported.
func (v *Validator) Error() error {
	// check errors
	if len(v.reports) > 0 {
		return ValidationError{Reports: v.reports}
	}

	return nil
}

// RuleContext carries the to be checked value.
type RuleContext struct {
	IValue interface{}
	RValue reflect.Value
}

// IsNil returns whether the value is nil.
func (c *RuleContext) IsNil() bool {
	// check plain nil
	if c.IValue == nil {
		return true
	}

	// check typed nils
	switch c.RValue.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map:
		return c.RValue.IsNil()
	}

	return false
}

// Unwrap will unwrap all pointers.
func (c *RuleContext) Unwrap() bool {
	// unwrap pointers
	var unwrapped bool
	for c.RValue.Kind() == reflect.Ptr {
		c.RValue = c.RValue.Elem()
		unwrapped = true
	}
	if unwrapped {
		c.IValue = c.RValue.Interface()
	}

	return unwrapped
}

func (c *RuleContext) Guard(fn func() error) error {
	// check nil
	if c.IsNil() {
		return nil
	}

	// unwrap
	c.Unwrap()

	return fn()
}

// Rule is a single validation rule.
type Rule func(ctx RuleContext) error

// Use will run the provided validation function.
func Use(fn func() error) Rule {
	return func(RuleContext) error {
		return fn()
	}
}

// IsNotZero will check if the provided value is not zero. It will determine
// zeroness using IsZero() or Zero() if implemented and default back to reflect.
// A nil pointer, slice, array or maps is also considered as zero.
func IsNotZero(ctx RuleContext) error {
	// check nil
	if ctx.IsNil() {
		return xo.SF("zero")
	}

	// check using IsValid method
	type isZero interface {
		IsZero() bool
	}
	if v, ok := ctx.IValue.(isZero); ok {
		// check zeroness
		if v.IsZero() {
			return xo.SF("zero")
		}

		return nil
	}

	// check using Valid method
	type zero interface {
		Zero() bool
	}
	if v, ok := ctx.IValue.(zero); ok {
		// check zeroness
		if v.Zero() {
			return xo.SF("zero")
		}

		return nil
	}

	// check zeroness
	if ctx.RValue.IsZero() {
		return xo.SF("zero")
	}

	return nil
}

// IsNotEmpty will check if the provided value is not empty. Emptiness can only
// be checked for strings, arrays, slices and maps.
func IsNotEmpty(ctx RuleContext) error {
	return ctx.Guard(func() error {
		// check array, slice, map and string length
		switch ctx.RValue.Kind() {
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			if ctx.RValue.Len() == 0 {
				return xo.SF("empty")
			}
		}

		// TODO: Check if string contains only whitespace?

		return nil
	})
}

// IsValid will check if the value is valid by calling IsValid() or Valid().
func IsValid(ctx RuleContext) error {
	// check using IsValid method
	type isValid interface {
		IsValid() bool
	}
	if v, ok := ctx.IValue.(isValid); ok {
		// check validity
		if !v.IsValid() {
			return xo.SF("invalid")
		}

		return nil
	}

	// check using Valid method
	type valid interface {
		Valid() bool
	}
	if v, ok := ctx.IValue.(valid); ok {
		// check validity
		if !v.Valid() {
			return xo.SF("invalid")
		}

		return nil
	}

	panic(fmt.Sprintf("stick: cannot check validity of %T", ctx.IValue))
}

// IsMinLen checks whether the value has at least the specified length.
func IsMinLen(min int) Rule {
	return func(ctx RuleContext) error {
		return ctx.Guard(func() error {
			// check length
			if ctx.RValue.Len() < min {
				return xo.SF("too short")
			}

			return nil
		})
	}
}

// IsMaxLen checks whether the value does not exceed the specified length.
func IsMaxLen(max int) Rule {
	return func(ctx RuleContext) error {
		// check length
		if ctx.RValue.Len() > max {
			return xo.SF("too long")
		}

		return nil
	}
}

// IsMinInt checks whether the value satisfies the provided minimum.
func IsMinInt(min int64) Rule {
	return func(ctx RuleContext) error {
		return ctx.Guard(func() error {
			// get number
			n, ok := GetInt(ctx.IValue)
			if !ok {
				panic("stick: expected int value")
			}

			// check min
			if n < min {
				return xo.SF("too small")
			}

			return nil
		})
	}
}

// IsMaxInt checks whether the value satisfies the provided maximum.
func IsMaxInt(max int64) Rule {
	return func(ctx RuleContext) error {
		return ctx.Guard(func() error {
			// get number
			n, ok := GetInt(ctx.IValue)
			if !ok {
				panic("stick: expected int value")
			}

			// check min
			if n > max {
				return xo.SF("too big")
			}

			return nil
		})
	}
}

// IsMinUint checks whether the value satisfies the provided minimum.
func IsMinUint(min uint64) Rule {
	return func(ctx RuleContext) error {
		return ctx.Guard(func() error {
			// get number
			n, ok := GetUint(ctx.IValue)
			if !ok {
				panic("stick: expected uint value")
			}

			// check range
			if n < min {
				return xo.SF("too small")
			}

			return nil
		})
	}
}

// IsMaxUint checks whether the value satisfies the provided maximum.
func IsMaxUint(max uint64) Rule {
	return func(ctx RuleContext) error {
		return ctx.Guard(func() error {
			// get number
			n, ok := GetUint(ctx.IValue)
			if !ok {
				panic("stick: expected uint value")
			}

			// check max
			if n > max {
				return xo.SF("too big")
			}

			return nil
		})
	}
}

// IsMinFloat checks whether the value satisfies the provided minimum.
func IsMinFloat(min float64) Rule {
	return func(ctx RuleContext) error {
		// get number
		n, ok := GetFloat(ctx.IValue)
		if !ok {
			panic("stick: expected float value")
		}

		// check min
		if n < min {
			return xo.SF("too small")
		}

		return nil
	}
}

// IsMaxFloat checks whether the value satisfies the provided maximum.
func IsMaxFloat(max float64) Rule {
	return func(ctx RuleContext) error {
		return ctx.Guard(func() error {
			// get number
			n, ok := GetFloat(ctx.IValue)
			if !ok {
				panic("stick: expected float value")
			}

			// check max
			if n > max {
				return xo.SF("too big")
			}

			return nil
		})
	}
}

// IsFormat will check of the value corresponds to the format determined by the
// provided string format checker.
func IsFormat(fn func(string) bool) Rule {
	return func(ctx RuleContext) error {
		return ctx.Guard(func() error {
			// get string
			str := ctx.IValue.(string)

			// check zero
			if str == "" {
				return nil
			}

			// check validity
			if !fn(str) {
				return xo.SF("invalid format")
			}

			return nil
		})
	}
}

// IsEmail will check if a string is a valid email.
var IsEmail = IsFormat(govalidator.IsEmail)

// IsURL will check if a string is a valid URL.
var IsURL = IsFormat(govalidator.IsURL)

// IsHost will check if a string is a valid host.
var IsHost = IsFormat(govalidator.IsHost)

// IsDNSName will check if a string is a valid DNS name.
var IsDNSName = IsFormat(govalidator.IsDNSName)

// IsIPAddress will check if a string is a valid IP address.
var IsIPAddress = IsFormat(govalidator.IsIP)

// IsNumeric will check if a string is numeric.
var IsNumeric = IsFormat(govalidator.IsNumeric)

// IsValidUTF8 will check if a string is valid utf8.
var IsValidUTF8 = IsFormat(utf8.ValidString)
