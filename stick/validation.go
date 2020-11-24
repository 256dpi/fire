package stick

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
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
type ValidationError map[error][]string

// Error implements the error interface.
func (e ValidationError) Error() string {
	// collect errors
	var errs []string
	for err, path := range e {
		errs = append(errs, fmt.Sprintf("%s: %s", strings.Join(path, "."), err.Error()))
	}

	// sort errors
	sort.Strings(errs)

	return strings.Join(errs, "; ")
}

// Validator is used to validate an object.
type Validator struct {
	obj   Accessible
	path  []string
	error ValidationError
}

// Validate will validate the provided accessible using the specified validator
// function.
func Validate(obj Accessible, fn func(v *Validator)) error {
	// prepare validator
	val := &Validator{obj: obj}

	// run validator
	fn(val)

	return xo.SW(val.Error())
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

	// prepare subject
	sub := Subject{
		IValue: value,
		RValue: reflect.ValueOf(value),
	}

	// handle optionals
	if optional {
		// skip if nil
		if sub.IsNil() {
			return
		}

		// otherwise unwrap pointer once
		sub.RValue = sub.RValue.Elem()
		sub.IValue = sub.RValue.Interface()
	}

	// execute rules
	for _, rule := range rules {
		err := rule(sub)
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

			// prepare subject
			sub := Subject{
				IValue: item.Interface(),
				RValue: item,
			}

			// execute rules
			for _, rule := range rules {
				err := rule(sub)
				if err != nil {
					v.Report(strconv.Itoa(i), err)
				}
			}
		}
	})
}

// Report will report a validation error.
func (v *Validator) Report(name string, err error) {
	// ensure error
	if v.error == nil {
		v.error = ValidationError{}
	}

	// copy path
	path := append([]string{}, v.path...)
	path = append(path, name)

	// add error
	v.error[err] = path
}

// Error will return the validation error or nil of no errors have yet been
// reported.
func (v *Validator) Error() error {
	// check error
	if v.error != nil {
		return v.error
	}

	return nil
}

// Subject carries the to be validated value.
type Subject struct {
	IValue interface{}
	RValue reflect.Value
}

// IsNil returns whether the value is nil.
func (s *Subject) IsNil() bool {
	// check plain nil
	if s.IValue == nil {
		return true
	}

	// check typed nils
	switch s.RValue.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map:
		return s.RValue.IsNil()
	}

	return false
}

// Unwrap will unwrap all pointers.
func (s *Subject) Unwrap() bool {
	// unwrap pointers
	var unwrapped bool
	for s.RValue.Kind() == reflect.Ptr {
		s.RValue = s.RValue.Elem()
		unwrapped = true
	}
	if unwrapped {
		s.IValue = s.RValue.Interface()
	}

	return unwrapped
}

// Guard will run the provided function if a concrete value can be unwrapped.
func (s *Subject) Guard(fn func() error) error {
	// check nil
	if s.IsNil() {
		return nil
	}

	// unwrap
	s.Unwrap()

	return fn()
}

// Rule is a single validation rule.
type Rule func(sub Subject) error

func isZero(sub Subject) bool {
	// check nil
	if sub.IsNil() {
		return true
	}

	// check using IsZero method
	type isZero interface {
		IsZero() bool
	}
	if v, ok := sub.IValue.(isZero); ok {
		// check zeroness
		if !v.IsZero() {
			return false
		}

		return true
	}

	// check using Zero method
	type zero interface {
		Zero() bool
	}
	if v, ok := sub.IValue.(zero); ok {
		// check zeroness
		if !v.Zero() {
			return false
		}

		return true
	}

	// unwrap pointer and check nil again if unwrapped
	if sub.Unwrap() && sub.IsNil() {
		return true
	}

	// check zeroness
	if !sub.RValue.IsZero() {
		return false
	}

	return true
}

// IsZero will check if the provided value is zero. It will determine zeroness
// using IsZero() or Zero() if implemented and default back to reflect. A nil
// pointer, slice, array or map is also considered as zero.
func IsZero(sub Subject) error {
	// check zeroness
	if !isZero(sub) {
		return xo.SF("not zero")
	}

	return nil
}

// IsNotZero will check if the provided value is not zero. It will determine
// zeroness using IsZero() or Zero() if implemented and default back to reflect.
// A nil pointer, slice, array or map is also considered as zero.
func IsNotZero(sub Subject) error {
	// check zeroness
	if isZero(sub) {
		return xo.SF("zero")
	}

	return nil
}

func isEmpty(sub Subject) bool {
	// check nil
	if sub.IsNil() {
		return true
	}

	// unwrap
	sub.Unwrap()

	// check array, slice, map and string length
	switch sub.RValue.Kind() {
	case reflect.Slice, reflect.Map:
		return sub.RValue.Len() == 0
	}

	panic(fmt.Sprintf("stick: cannot check length of %T", sub.IValue))
}

// IsEmpty will check if the provided value is empty. Emptiness can only be
// checked for slices and maps.
func IsEmpty(sub Subject) error {
	// check emptiness
	if !isEmpty(sub) {
		return xo.SF("not empty")
	}

	return nil
}

// IsNotEmpty will check if the provided value is not empty. Emptiness can only
// be checked for slices and maps.
func IsNotEmpty(sub Subject) error {
	// check emptiness
	if isEmpty(sub) {
		return xo.SF("empty")
	}

	return nil
}

// IsValid will check if the value is valid by calling Validate(), IsValid() or
// Valid().
func IsValid(sub Subject) error {
	// check using Validate() method
	if v, ok := sub.IValue.(Validatable); ok {
		return v.Validate()
	}

	// check using IsValid() method
	type isValid interface {
		IsValid() bool
	}
	if v, ok := sub.IValue.(isValid); ok {
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
	if v, ok := sub.IValue.(valid); ok {
		// check validity
		if !v.Valid() {
			return xo.SF("invalid")
		}

		return nil
	}

	panic(fmt.Sprintf("stick: cannot check validity of %T", sub.IValue))
}

// IsMinLen checks whether the value has at least the specified length.
func IsMinLen(min int) Rule {
	return func(sub Subject) error {
		return sub.Guard(func() error {
			// check length
			if sub.RValue.Len() < min {
				return xo.SF("too short")
			}

			return nil
		})
	}
}

// IsMaxLen checks whether the value does not exceed the specified length.
func IsMaxLen(max int) Rule {
	return func(sub Subject) error {
		// check length
		if sub.RValue.Len() > max {
			return xo.SF("too long")
		}

		return nil
	}
}

// IsMinInt checks whether the value satisfies the provided minimum.
func IsMinInt(min int64) Rule {
	return func(sub Subject) error {
		return sub.Guard(func() error {
			// check value
			switch sub.RValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			default:
				panic("stick: expected int value")
			}

			// check min
			if sub.RValue.Int() < min {
				return xo.SF("too small")
			}

			return nil
		})
	}
}

// IsMaxInt checks whether the value satisfies the provided maximum.
func IsMaxInt(max int64) Rule {
	return func(sub Subject) error {
		return sub.Guard(func() error {
			// check value
			switch sub.RValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			default:
				panic("stick: expected int value")
			}

			// check min
			if sub.RValue.Int() > max {
				return xo.SF("too big")
			}

			return nil
		})
	}
}

// IsMinUint checks whether the value satisfies the provided minimum.
func IsMinUint(min uint64) Rule {
	return func(sub Subject) error {
		return sub.Guard(func() error {
			// check value
			switch sub.RValue.Kind() {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			default:
				panic("stick: expected uint value")
			}

			// check range
			if sub.RValue.Uint() < min {
				return xo.SF("too small")
			}

			return nil
		})
	}
}

// IsMaxUint checks whether the value satisfies the provided maximum.
func IsMaxUint(max uint64) Rule {
	return func(sub Subject) error {
		return sub.Guard(func() error {
			// check value
			switch sub.RValue.Kind() {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			default:
				panic("stick: expected uint value")
			}

			// check max
			if sub.RValue.Uint() > max {
				return xo.SF("too big")
			}

			return nil
		})
	}
}

// IsMinFloat checks whether the value satisfies the provided minimum.
func IsMinFloat(min float64) Rule {
	return func(sub Subject) error {
		// check value
		switch sub.RValue.Kind() {
		case reflect.Float32, reflect.Float64:
		default:
			panic("stick: expected float value")
		}

		// check min
		if sub.RValue.Float() < min {
			return xo.SF("too small")
		}

		return nil
	}
}

// IsMaxFloat checks whether the value satisfies the provided maximum.
func IsMaxFloat(max float64) Rule {
	return func(sub Subject) error {
		return sub.Guard(func() error {
			// check value
			switch sub.RValue.Kind() {
			case reflect.Float32, reflect.Float64:
			default:
				panic("stick: expected float value")
			}

			// check max
			if sub.RValue.Float() > max {
				return xo.SF("too big")
			}

			return nil
		})
	}
}

// IsFormat will check of the value corresponds to the format determined by the
// provided string format checker.
func IsFormat(fns ...func(string) bool) Rule {
	return func(sub Subject) error {
		return sub.Guard(func() error {
			// check value
			if sub.RValue.Kind() != reflect.String {
				panic("stick: expected string value")
			}

			// get string
			str := sub.RValue.String()

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
