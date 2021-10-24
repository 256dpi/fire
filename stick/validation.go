package stick

import (
	"errors"
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
	// collect messages
	var messages []string
	for err, path := range e {
		// get message
		msg := "error"
		if xo.IsSafe(err) {
			msg = err.Error()
		}

		// add message
		messages = append(messages, fmt.Sprintf("%s: %s", strings.Join(path, "."), msg))
	}

	// sort messages
	sort.Strings(messages)

	// combine messages
	err := strings.Join(messages, "; ")

	return err
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

	return val.Error()
}

// Nest will nest validations under the specified field.
func (v *Validator) Nest(field string, fn func()) {
	// push
	v.path = append(v.path, field)

	// yield
	fn()

	// pop
	v.path = v.path[:len(v.path)-1]
}

// Value will validate the value at the named field using the provided rules.
// Pointer may be optional and are skipped if nil or unwrapped if present.
func (v *Validator) Value(name string, optional bool, rules ...Rule) {
	// get value
	value := MustGetRaw(v.obj, name)

	// prepare subject
	sub := Subject{
		IValue: value.Interface(),
		RValue: value,
	}

	// handle optionals
	if optional {
		// check kind
		if sub.RValue.Kind() != reflect.Ptr {
			panic("stick: expected pointer")
		}

		// skip if nil
		if sub.RValue.IsNil() {
			return
		}

		// otherwise, unwrap pointer once
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

// Items will validate each item of the array/slice at the named field using the
// provided rules.
func (v *Validator) Items(name string, rules ...Rule) {
	// get slice
	slice := reflect.ValueOf(MustGet(v.obj, name))

	// check type
	if slice.Kind() != reflect.Slice && slice.Kind() != reflect.Array {
		panic("stick: expected array/slice")
	}

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

	// check error
	var valError ValidationError
	if errors.As(err, &valError) {
		for err, pth := range valError {
			// copy path
			path := append([]string{}, v.path...)
			path = append(path, name)
			path = append(path, pth...)

			// add error
			v.error[err] = path
		}
		return
	}

	// copy path
	path := append([]string{}, v.path...)
	path = append(path, name)

	// add error
	v.error[xo.W(err)] = path
}

// Error will return the validation error or nil of no errors have yet been
// reported.
func (v *Validator) Error() error {
	// check error
	if v.error != nil {
		return xo.SW(v.error)
	}

	return nil
}

// Subject carries the to be validated value.
type Subject struct {
	IValue interface{}
	RValue reflect.Value
}

// IsNil returns true if the value is nil or a typed nil (zero pointer).
func (s *Subject) IsNil() bool {
	// check plain nil
	if s.IValue == nil {
		return true
	}

	// check typed nils
	switch s.RValue.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
		return s.RValue.IsNil()
	}

	return false
}

// Unwrap will unwrap all pointers and return whether a value is available.
func (s *Subject) Unwrap() bool {
	// unwrap pointers
	var unwrapped bool
	for s.RValue.Kind() == reflect.Ptr {
		if s.RValue.IsNil() {
			return false
		}
		s.RValue = s.RValue.Elem()
		unwrapped = true
	}
	if unwrapped {
		s.IValue = s.RValue.Interface()
	}

	return true
}

// Reference will attempt to obtain a referenced subject for interface testing.
// Only struct fields and array/slice items can be referenced.
func (s Subject) Reference() (Subject, bool) {
	// check if already pointer
	if s.RValue.Kind() == reflect.Ptr {
		return s, false
	}

	// check address
	if !s.RValue.CanAddr() {
		return s, false
	}

	// set value
	s.RValue = s.RValue.Addr()
	s.IValue = s.RValue.Interface()

	return s, true
}

// Rule is a single validation rule.
type Rule func(sub Subject) error

// IsZero will check if the provided value is zero. It will determine zeroness
// using IsZero() or Zero() if implemented. A nil pointer, slice, array or map
// is also considered as zero.
func IsZero(sub Subject) error {
	// check zeroness
	if !isZero(sub) {
		return xo.SF("not zero")
	}

	return nil
}

// IsNotZero will check if the provided value is not zero. It will determine
// zeroness using IsZero() or Zero() if implemented. A nil pointer, slice, array
// or map is also considered as zero.
func IsNotZero(sub Subject) error {
	// check zeroness
	if isZero(sub) {
		return xo.SF("zero")
	}

	return nil
}

func isZero(sub Subject) bool {
	// check nil
	if sub.IsNil() {
		return true
	}

	// get reference
	ref, _ := sub.Reference()

	// check using IsZero method
	type isZero interface {
		IsZero() bool
	}
	if v, ok := sub.IValue.(isZero); ok {
		return v.IsZero()
	} else if v, ok := ref.IValue.(isZero); ok {
		return v.IsZero()
	}

	// check using Zero method
	type zero interface {
		Zero() bool
	}
	if v, ok := sub.IValue.(zero); ok {
		return v.Zero()
	} else if v, ok := ref.IValue.(zero); ok {
		return v.Zero()
	}

	// unwrap
	if !sub.Unwrap() {
		return true
	}

	// check zeroness
	return sub.RValue.IsZero()
}

// IsEmpty will check if the provided value is empty. Emptiness can only be
// checked for slices and maps.
func IsEmpty(sub Subject) error {
	// unwrap
	if !sub.Unwrap() {
		return nil
	}

	// check emptiness
	if !isEmpty(sub) {
		return xo.SF("not empty")
	}

	return nil
}

// IsNotEmpty will check if the provided value is not empty. Emptiness can only
// be checked for slices and maps.
func IsNotEmpty(sub Subject) error {
	// unwrap
	if !sub.Unwrap() {
		return nil
	}

	// check emptiness
	if isEmpty(sub) {
		return xo.SF("empty")
	}

	return nil
}

func isEmpty(sub Subject) bool {
	// check slice and map length
	switch sub.RValue.Kind() {
	case reflect.Slice, reflect.Map, reflect.String:
		return sub.RValue.Len() == 0
	}

	panic(fmt.Sprintf("stick: cannot check length of %T", sub.IValue))
}

// IsValid will check if the value is valid by calling Validate(), IsValid() or
// Valid().
func IsValid(sub Subject) error {
	// check nil
	if sub.IsNil() {
		return nil
	}

	// check raw
	ok, err := isValid(sub.IValue)
	if ok {
		return err
	}

	// check reference
	if ref, refOK := sub.Reference(); refOK {
		ok, err = isValid(ref.IValue)
		if ok {
			return err
		}
	}

	panic(fmt.Sprintf("stick: cannot check validity of %T", sub.IValue))
}

func isValid(val interface{}) (bool, error) {
	// check using Validate() method
	if v, ok := val.(Validatable); ok {
		return true, v.Validate()
	}

	// check using IsValid() method
	type isValid interface {
		IsValid() bool
	}
	if v, ok := val.(isValid); ok {
		// check validity
		if !v.IsValid() {
			return true, xo.SF("invalid")
		}

		return true, nil
	}

	// check using Valid() method
	type valid interface {
		Valid() bool
	}
	if v, ok := val.(valid); ok {
		// check validity
		if !v.Valid() {
			return true, xo.SF("invalid")
		}

		return true, nil
	}

	return false, nil
}

// IsMinLen checks whether the value has at least the specified length.
func IsMinLen(min int) Rule {
	return func(sub Subject) error {
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

		// check value
		switch sub.RValue.Kind() {
		case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		default:
			panic("stick: expected array/map/slice/string value")
		}

		// check length
		if sub.RValue.Len() < min {
			return xo.SF("too short")
		}

		return nil
	}
}

// IsMaxLen checks whether the value does not exceed the specified length.
func IsMaxLen(max int) Rule {
	return func(sub Subject) error {
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

		// check value
		switch sub.RValue.Kind() {
		case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		default:
			panic("stick: expected array/map/slice/string value")
		}

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
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

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
	}
}

// IsMaxInt checks whether the value satisfies the provided maximum.
func IsMaxInt(max int64) Rule {
	return func(sub Subject) error {
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

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
	}
}

// IsMinUint checks whether the value satisfies the provided minimum.
func IsMinUint(min uint64) Rule {
	return func(sub Subject) error {
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

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
	}
}

// IsMaxUint checks whether the value satisfies the provided maximum.
func IsMaxUint(max uint64) Rule {
	return func(sub Subject) error {
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

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
	}
}

// IsMinFloat checks whether the value satisfies the provided minimum.
func IsMinFloat(min float64) Rule {
	return func(sub Subject) error {
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

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
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

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
	}
}

// IsFormat will check of the value corresponds to the format determined by the
// provided string format checker.
func IsFormat(fns ...func(string) bool) Rule {
	return func(sub Subject) error {
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

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
