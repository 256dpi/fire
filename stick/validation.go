package stick

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"
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

// Guard will run the provided function if a concrete value can be unwrapped.
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

// IsOK will run the provided validation function.
func IsOK(fn func() error) Rule {
	return func(RuleContext) error {
		return fn()
	}
}

// IsNot will return an error with the provided message if the specified rules
// passes.
func IsNot(msg string, rule Rule) Rule {
	return func(ctx RuleContext) error {
		// check rule
		if rule(ctx) == nil {
			return xo.SF(msg)
		}

		return nil
	}
}

// IsZero will check if the provided value is zero. It will determine zeroness
// using IsZero() or Zero() if implemented and default back to reflect. A nil
// pointer, slice, array or map is also considered as zero.
func IsZero(ctx RuleContext) error {
	// check nil
	if ctx.IsNil() {
		return nil
	}

	// check using IsZero method
	type isZero interface {
		IsZero() bool
	}
	if v, ok := ctx.IValue.(isZero); ok {
		// check zeroness
		if !v.IsZero() {
			return xo.SF("not zero")
		}

		return nil
	}

	// check using Zero method
	type zero interface {
		Zero() bool
	}
	if v, ok := ctx.IValue.(zero); ok {
		// check zeroness
		if !v.Zero() {
			return xo.SF("not zero")
		}

		return nil
	}

	// unwrap pointer
	ctx.Unwrap()

	// check nil again
	if ctx.IsNil() {
		return nil
	}

	// check zeroness
	if !ctx.RValue.IsZero() {
		return xo.SF("not zero")
	}

	return nil
}

// IsNotZero inverts the IsZero rule.
var IsNotZero = IsNot("zero", IsZero)

// IsEmpty will check if the provided value is empty. Emptiness can only be
// checked for slices and maps.
func IsEmpty(ctx RuleContext) error {
	return ctx.Guard(func() error {
		// check array, slice, map and string length
		switch ctx.RValue.Kind() {
		case reflect.Slice, reflect.Map:
			if ctx.RValue.Len() != 0 {
				return xo.SF("not empty")
			}
		}

		return nil
	})
}

// IsNotEmpty inverts the IsEmpty rule.
var IsNotEmpty = IsNot("empty", IsEmpty)

// IsValid will check if the value is valid by calling Validate(), IsValid() or
// Valid().
func IsValid(ctx RuleContext) error {
	// check using Validate() method
	if v, ok := ctx.IValue.(Validatable); ok {
		return v.Validate()
	}

	// check using IsValid() method
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

	// check using Valid() method
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
			// check value
			switch ctx.RValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			default:
				panic("stick: expected int value")
			}

			// check min
			if ctx.RValue.Int() < min {
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
			// check value
			switch ctx.RValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			default:
				panic("stick: expected int value")
			}

			// check min
			if ctx.RValue.Int() > max {
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
			// check value
			switch ctx.RValue.Kind() {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			default:
				panic("stick: expected uint value")
			}

			// check range
			if ctx.RValue.Uint() < min {
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
			// check value
			switch ctx.RValue.Kind() {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			default:
				panic("stick: expected uint value")
			}

			// check max
			if ctx.RValue.Uint() > max {
				return xo.SF("too big")
			}

			return nil
		})
	}
}

// IsMinFloat checks whether the value satisfies the provided minimum.
func IsMinFloat(min float64) Rule {
	return func(ctx RuleContext) error {
		// check value
		switch ctx.RValue.Kind() {
		case reflect.Float32, reflect.Float64:
		default:
			panic("stick: expected float value")
		}

		// check min
		if ctx.RValue.Float() < min {
			return xo.SF("too small")
		}

		return nil
	}
}

// IsMaxFloat checks whether the value satisfies the provided maximum.
func IsMaxFloat(max float64) Rule {
	return func(ctx RuleContext) error {
		return ctx.Guard(func() error {
			// check value
			switch ctx.RValue.Kind() {
			case reflect.Float32, reflect.Float64:
			default:
				panic("stick: expected float value")
			}

			// check max
			if ctx.RValue.Float() > max {
				return xo.SF("too big")
			}

			return nil
		})
	}
}

// IsFormat will check of the value corresponds to the format determined by the
// provided string format checker.
func IsFormat(fns ...func(string) bool) Rule {
	return func(ctx RuleContext) error {
		return ctx.Guard(func() error {
			// check value
			if ctx.RValue.Kind() != reflect.String {
				panic("stick: expected string value")
			}

			// get string
			str := ctx.RValue.String()

			// check zero
			if str == "" {
				return nil
			}

			// check validity
			for _, fn := range fns {
				if !fn(str) {
					return xo.SF("invalid format")
				}
			}

			return nil
		})
	}
}

// IsRegexMatch will check if a string matches a regular expression.
func IsRegexMatch(reg *regexp.Regexp) Rule {
	return IsFormat(reg.MatchString)
}

// IsPatternMatch will check if a string matches a regular expression pattern.
func IsPatternMatch(pattern string) Rule {
	return IsRegexMatch(regexp.MustCompile(pattern))
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

// IsVisible will check if a string is visible.
var IsVisible = IsFormat(utf8.ValidString, func(s string) bool {
	// count characters and whitespace
	c := 0
	w := 0
	for _, r := range s {
		c++
		if unicode.IsSpace(r) {
			w++
		}
	}

	return w < c
})
