package stick

import (
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"
)

type noValidation struct {
	NoValidation
}

func TestNoValidation(t *testing.T) {
	var nv noValidation
	assert.NoError(t, nv.Validate())
}

type subValidatable struct {
	String string
}

func (v *subValidatable) Validate() error {
	if v.String != "valid" {
		return xo.SF("invalid")
	}

	return nil
}

type validatable struct {
	Int            int32
	Uint           uint16
	Float          float64
	OptInt         *int
	String         string
	OptString      *string
	Strings        []string
	Time           time.Time  // never zero
	OptTime        *time.Time // never zero
	Validatable    subValidatable
	OptValidatable *subValidatable
	Validatables   []subValidatable
}

func (v *validatable) Validate() error {
	if v.String != "valid" {
		return xo.SF("invalid")
	}

	return nil
}

func TestValidate(t *testing.T) {
	obj := &validatable{}

	assert.PanicsWithValue(t, `stick: could not get field "Foo" on "stick.validatable"`, func() {
		_ = Validate(obj, func(v *Validator) {
			v.Value("Foo", false)
		})
	})

	err := Validate(obj, func(v *Validator) {
		v.Value("String", false)
	})
	assert.NoError(t, err)

	err = Validate(obj, func(v *Validator) {
		v.Value("String", false, IsMinLen(5))
	})
	assert.Error(t, err)
	assert.Equal(t, "String: too short", err.Error())

	err = Validate(obj, func(v *Validator) {
		v.Value("Int", false, IsMinInt(5))
		v.Value("Uint", false, IsMinUint(5))
	})
	assert.Error(t, err)
	assert.Equal(t, "Int: too small; Uint: too small", err.Error())

	assert.PanicsWithValue(t, "stick: expected pointer", func() {
		err = Validate(obj, func(v *Validator) {
			v.Value("String", true)
		})
	})

	err = Validate(obj, func(v *Validator) {
		v.Value("OptInt", true, IsMinInt(5))
	})
	assert.NoError(t, err)

	i := 3
	obj.OptInt = &i
	err = Validate(obj, func(v *Validator) {
		v.Value("OptInt", true, IsMinInt(5))
	})
	assert.Error(t, err)
	assert.Equal(t, "OptInt: too small", err.Error())

	assert.PanicsWithValue(t, "stick: expected array/slice", func() {
		err = Validate(obj, func(v *Validator) {
			v.Items("String")
		})
	})

	obj.Strings = []string{""}
	err = Validate(obj, func(v *Validator) {
		v.Value("Strings", false, IsMinLen(5))
		v.Items("Strings", IsMinLen(5))
	})
	assert.Error(t, err)
	assert.Equal(t, "Strings.0: too short; Strings: too short", err.Error())

	err = Validate(obj, func(v *Validator) {
		v.Report("Foo", xo.F("some error"))
	})
	assert.Error(t, err)
	assert.Equal(t, "Foo: error", err.Error())

	obj.OptValidatable = &subValidatable{}
	obj.Validatables = []subValidatable{
		{},
	}
	err = Validate(obj, func(v *Validator) {
		v.Value("Validatable", false, IsValid)
		v.Value("OptValidatable", true, IsValid)
		v.Items("Validatables", IsValid)
		v.Value("Validatables", false, IsMinLen(5))
	})
	assert.Error(t, err)
	assert.Equal(t, "OptValidatable: invalid; Validatable: invalid; Validatables.0: invalid; Validatables: too short", err.Error())
}

func TestValidateErrorIsolation(t *testing.T) {
	err := Validate(nil, func(v *Validator) {
		v.Report("Foo", io.EOF)
		v.Report("Bar", io.EOF)
	})
	assert.Error(t, err)
	assert.Equal(t, "Bar: error; Foo: error", err.Error())
}

func TestValidateErrorMerge(t *testing.T) {
	err := Validate(nil, func(v *Validator) {
		v.Report("Foo", Validate(nil, func(v *Validator) {
			v.Report("Bar", io.EOF)
		}))
	})
	assert.Error(t, err)
	assert.Equal(t, "Foo.Bar: error", err.Error())
}

func TestSubjectUnwrap(t *testing.T) {
	i1 := 1
	ruleTest(t, &i1, IsMaxInt(5), "")

	var i2 *int
	ruleTest(t, i2, IsMaxInt(5), "")
	ruleTest(t, &i2, IsMaxInt(5), "")
}

func ruleTest(t *testing.T, v interface{}, rule Rule, msg string) {
	// prepare addressable value
	val := reflect.ValueOf(v)
	ptr := reflect.New(val.Type())
	ptr.Elem().Set(val)

	// prepare subject
	sub := Subject{
		IValue: ptr.Elem().Interface(),
		RValue: ptr.Elem(),
	}

	// test rule
	err := rule(sub)
	if msg == "" {
		assert.NoError(t, err)
	} else {
		if assert.Error(t, err) {
			assert.Equal(t, msg, err.Error())
		}
	}
}

func ptr(v interface{}) interface{} {
	val := reflect.ValueOf(v)
	ptr := reflect.New(val.Type())
	ptr.Elem().Set(val)
	return ptr.Interface()
}

func TestIsEqual(t *testing.T) {
	ruleTest(t, "", IsEqual(""), "")
	ruleTest(t, "", IsEqual("foo"), "not equal")
}

type zeroStr string

func (s zeroStr) Zero() bool {
	return s == "zero"
}

type zeroStrPtr string

func (s *zeroStrPtr) Zero() bool {
	return *s == "zero"
}

type isZeroStr string

func (s isZeroStr) IsZero() bool {
	return s == "zero"
}

type isZeroStrPtr string

func (s *isZeroStrPtr) IsZero() bool {
	return *s == "zero"
}

func TestIsZero(t *testing.T) {
	ruleTest(t, "", IsZero, "")
	ruleTest(t, "foo", IsZero, "not zero")
	ruleTest(t, (*string)(nil), IsZero, "")
	ruleTest(t, ptr(""), IsZero, "")
	ruleTest(t, ptr("foo"), IsZero, "not zero")

	ruleTest(t, time.Time{}, IsZero, "")
	ruleTest(t, time.Now(), IsZero, "not zero")
	ruleTest(t, (*time.Time)(nil), IsZero, "")
	ruleTest(t, &time.Time{}, IsZero, "")
	ruleTest(t, ptr(time.Now()), IsZero, "not zero")

	ruleTest(t, zeroStr(""), IsZero, "not zero")
	ruleTest(t, zeroStr("zero"), IsZero, "")
	ruleTest(t, (*zeroStr)(nil), IsZero, "")
	ruleTest(t, ptr(zeroStr("")), IsZero, "not zero")
	ruleTest(t, ptr(zeroStr("zero")), IsZero, "")

	ruleTest(t, zeroStrPtr(""), IsZero, "not zero")
	ruleTest(t, zeroStrPtr("zero"), IsZero, "")
	ruleTest(t, (*zeroStrPtr)(nil), IsZero, "")
	ruleTest(t, ptr(zeroStrPtr("")), IsZero, "not zero")
	ruleTest(t, ptr(zeroStrPtr("zero")), IsZero, "")

	ruleTest(t, isZeroStr(""), IsZero, "not zero")
	ruleTest(t, isZeroStr("zero"), IsZero, "")
	ruleTest(t, (*isZeroStr)(nil), IsZero, "")
	ruleTest(t, ptr(isZeroStr("")), IsZero, "not zero")
	ruleTest(t, ptr(isZeroStr("zero")), IsZero, "")

	ruleTest(t, isZeroStrPtr(""), IsZero, "not zero")
	ruleTest(t, isZeroStrPtr("zero"), IsZero, "")
	ruleTest(t, (*isZeroStrPtr)(nil), IsZero, "")
	ruleTest(t, ptr(isZeroStrPtr("")), IsZero, "not zero")
	ruleTest(t, ptr(isZeroStrPtr("zero")), IsZero, "")
}

func TestIsNotZero(t *testing.T) {
	ruleTest(t, "", IsNotZero, "zero")
	ruleTest(t, "foo", IsNotZero, "")
	ruleTest(t, (*string)(nil), IsNotZero, "zero")
	ruleTest(t, ptr(""), IsNotZero, "zero")
	ruleTest(t, ptr("foo"), IsNotZero, "")

	ruleTest(t, time.Time{}, IsNotZero, "zero")
	ruleTest(t, time.Now(), IsNotZero, "")
	ruleTest(t, (*time.Time)(nil), IsNotZero, "zero")
	ruleTest(t, &time.Time{}, IsNotZero, "zero")
	ruleTest(t, ptr(time.Now()), IsNotZero, "")

	ruleTest(t, zeroStr(""), IsNotZero, "")
	ruleTest(t, zeroStr("zero"), IsNotZero, "zero")
	ruleTest(t, (*zeroStr)(nil), IsNotZero, "zero")
	ruleTest(t, ptr(zeroStr("")), IsNotZero, "")
	ruleTest(t, ptr(zeroStr("zero")), IsNotZero, "zero")

	ruleTest(t, zeroStrPtr(""), IsNotZero, "")
	ruleTest(t, zeroStrPtr("zero"), IsNotZero, "zero")
	ruleTest(t, (*zeroStrPtr)(nil), IsNotZero, "zero")
	ruleTest(t, ptr(zeroStrPtr("")), IsNotZero, "")
	ruleTest(t, ptr(zeroStrPtr("zero")), IsNotZero, "zero")

	ruleTest(t, isZeroStr(""), IsNotZero, "")
	ruleTest(t, isZeroStr("zero"), IsNotZero, "zero")
	ruleTest(t, (*isZeroStr)(nil), IsNotZero, "zero")
	ruleTest(t, ptr(isZeroStr("")), IsNotZero, "")
	ruleTest(t, ptr(isZeroStr("zero")), IsNotZero, "zero")

	ruleTest(t, isZeroStrPtr(""), IsNotZero, "")
	ruleTest(t, isZeroStrPtr("zero"), IsNotZero, "zero")
	ruleTest(t, (*isZeroStrPtr)(nil), IsNotZero, "zero")
	ruleTest(t, ptr(isZeroStrPtr("")), IsNotZero, "")
	ruleTest(t, ptr(isZeroStrPtr("zero")), IsNotZero, "zero")
}

func TestEmpty(t *testing.T) {
	assert.PanicsWithValue(t, `stick: cannot check length of int`, func() {
		ruleTest(t, 1, IsEmpty, "")
	})

	ruleTest(t, (*string)(nil), IsEmpty, "")
	ruleTest(t, "", IsEmpty, "")
	ruleTest(t, "foo", IsEmpty, "not empty")

	ruleTest(t, (*[]byte)(nil), IsEmpty, "")
	ruleTest(t, ([]byte)(nil), IsEmpty, "")
	ruleTest(t, []byte{}, IsEmpty, "")
	ruleTest(t, []byte{1}, IsEmpty, "not empty")

	ruleTest(t, (*Map)(nil), IsEmpty, "")
	ruleTest(t, (Map)(nil), IsEmpty, "")
	ruleTest(t, Map{}, IsEmpty, "")
	ruleTest(t, Map{"k": "v"}, IsEmpty, "not empty")
}

func TestNotEmpty(t *testing.T) {
	assert.PanicsWithValue(t, `stick: cannot check length of int`, func() {
		ruleTest(t, 1, IsNotEmpty, "")
	})

	ruleTest(t, (*string)(nil), IsNotEmpty, "")
	ruleTest(t, "", IsNotEmpty, "empty")
	ruleTest(t, "foo", IsNotEmpty, "")

	ruleTest(t, (*[]byte)(nil), IsNotEmpty, "")
	ruleTest(t, ([]byte)(nil), IsNotEmpty, "empty")
	ruleTest(t, []byte{}, IsNotEmpty, "empty")
	ruleTest(t, []byte{1}, IsNotEmpty, "")

	ruleTest(t, (*Map)(nil), IsNotEmpty, "")
	ruleTest(t, (Map)(nil), IsNotEmpty, "empty")
	ruleTest(t, Map{}, IsNotEmpty, "empty")
	ruleTest(t, Map{"k": "v"}, IsNotEmpty, "")
}

type validStr string

func (s validStr) Valid() bool {
	return s == "valid"
}

type validStrPtr string

func (s *validStrPtr) Valid() bool {
	return *s == "valid"
}

type isValidStr string

func (s isValidStr) IsValid() bool {
	return s == "valid"
}

type isValidStrPtr string

func (s *isValidStrPtr) IsValid() bool {
	return *s == "valid"
}

func TestIsValid(t *testing.T) {
	assert.PanicsWithValue(t, `stick: cannot check validity of string`, func() {
		ruleTest(t, "", IsValid, "")
	})

	assert.PanicsWithValue(t, "stick: cannot check validity of []uint8", func() {
		ruleTest(t, ([]byte)(nil), IsValid, "")
	})

	ruleTest(t, validatable{}, IsValid, "invalid")
	ruleTest(t, validatable{String: "valid"}, IsValid, "")
	ruleTest(t, (*validatable)(nil), IsValid, "")
	ruleTest(t, &validatable{}, IsValid, "invalid")
	ruleTest(t, &validatable{String: "valid"}, IsValid, "")

	ruleTest(t, validStr(""), IsValid, "invalid")
	ruleTest(t, validStr("valid"), IsValid, "")
	ruleTest(t, (*validStr)(nil), IsValid, "")
	ruleTest(t, ptr(validStr("")), IsValid, "invalid")
	ruleTest(t, ptr(validStr("valid")), IsValid, "")

	ruleTest(t, validStrPtr(""), IsValid, "invalid")
	ruleTest(t, validStrPtr("valid"), IsValid, "")
	ruleTest(t, (*validStrPtr)(nil), IsValid, "")
	ruleTest(t, ptr(validStrPtr("")), IsValid, "invalid")
	ruleTest(t, ptr(validStrPtr("valid")), IsValid, "")

	ruleTest(t, isValidStr(""), IsValid, "invalid")
	ruleTest(t, isValidStr("valid"), IsValid, "")
	ruleTest(t, (*isValidStr)(nil), IsValid, "")
	ruleTest(t, ptr(isValidStr("")), IsValid, "invalid")
	ruleTest(t, ptr(isValidStr("valid")), IsValid, "")

	ruleTest(t, isValidStrPtr(""), IsValid, "invalid")
	ruleTest(t, isValidStrPtr("valid"), IsValid, "")
	ruleTest(t, (*isValidStrPtr)(nil), IsValid, "")
	ruleTest(t, ptr(isValidStrPtr("")), IsValid, "invalid")
	ruleTest(t, ptr(isValidStrPtr("valid")), IsValid, "")
}

func TestIsMinLen(t *testing.T) {
	assert.PanicsWithValue(t, "stick: expected array/map/slice/string value", func() {
		ruleTest(t, 1, IsMinLen(5), "")
	})
	ruleTest(t, (*string)(nil), IsMinLen(5), "")
	ruleTest(t, "", IsMinLen(5), "too short")
	ruleTest(t, "Hello World!", IsMinLen(5), "")
}

func TestIsMaxLen(t *testing.T) {
	assert.PanicsWithValue(t, "stick: expected array/map/slice/string value", func() {
		ruleTest(t, 1, IsMaxLen(5), "")
	})
	ruleTest(t, (*string)(nil), IsMaxLen(5), "")
	ruleTest(t, "", IsMaxLen(5), "")
	ruleTest(t, "Hello World!", IsMaxLen(5), "too long")
}

func TestIsMin(t *testing.T) {
	assert.PanicsWithValue(t, "stick: expected int value", func() {
		ruleTest(t, uint(1), IsMinInt(1), "")
	})
	ruleTest(t, (*int)(nil), IsMinInt(5), "")
	ruleTest(t, 7, IsMinInt(5), "")
	ruleTest(t, int16(1), IsMinInt(5), "too small")

	assert.PanicsWithValue(t, "stick: expected uint value", func() {
		ruleTest(t, 1, IsMinUint(1), "")
	})
	ruleTest(t, (*uint)(nil), IsMinUint(5), "")
	ruleTest(t, uint(7), IsMinUint(5), "")
	ruleTest(t, uint16(1), IsMinUint(5), "too small")

	assert.PanicsWithValue(t, "stick: expected float value", func() {
		ruleTest(t, 1, IsMinFloat(1), "")
	})
	ruleTest(t, (*float32)(nil), IsMinFloat(5), "")
	ruleTest(t, 7., IsMinFloat(5), "")
	ruleTest(t, float32(1), IsMinFloat(5), "too small")
}

func TestIsMax(t *testing.T) {
	assert.PanicsWithValue(t, "stick: expected int value", func() {
		ruleTest(t, uint(1), IsMaxInt(1), "")
	})
	ruleTest(t, (*int)(nil), IsMaxInt(5), "")
	ruleTest(t, 1, IsMaxInt(5), "")
	ruleTest(t, int16(7), IsMaxInt(5), "too big")

	assert.PanicsWithValue(t, "stick: expected uint value", func() {
		ruleTest(t, 1, IsMaxUint(1), "")
	})
	ruleTest(t, (*uint)(nil), IsMaxUint(5), "")
	ruleTest(t, uint(1), IsMaxUint(5), "")
	ruleTest(t, uint16(7), IsMaxUint(5), "too big")

	assert.PanicsWithValue(t, "stick: expected float value", func() {
		ruleTest(t, 1, IsMaxFloat(1), "")
	})
	ruleTest(t, (*float32)(nil), IsMaxFloat(5), "")
	ruleTest(t, 1., IsMaxFloat(5), "")
	ruleTest(t, float32(7), IsMaxFloat(5), "too big")
}

func TestIsFormat(t *testing.T) {
	assert.PanicsWithValue(t, `stick: expected string value`, func() {
		ruleTest(t, 1, IsEmail, "")
	})

	ruleTest(t, (*string)(nil), IsEmail, "")

	ruleTest(t, "", IsPatternMatch("\\d+"), "")
	ruleTest(t, "-", IsPatternMatch("\\d+"), "invalid format")
	ruleTest(t, "7", IsPatternMatch("\\d+"), "")

	ruleTest(t, "", IsEmail, "")
	ruleTest(t, "-", IsEmail, "invalid format")
	ruleTest(t, "foo@bar.com", IsEmail, "")

	ruleTest(t, "", IsURL(false), "")
	ruleTest(t, "-", IsURL(false), "invalid format")
	ruleTest(t, "foo.bar/baz", IsURL(false), "")
	ruleTest(t, "foo.bar/baz", IsURL(true), "invalid format")
	ruleTest(t, "https://foo.bar/baz", IsURL(false), "")
	ruleTest(t, "https://foo.bar/baz", IsURL(true), "")

	ruleTest(t, "", IsHost, "")
	ruleTest(t, "-", IsHost, "invalid format")
	ruleTest(t, "foo.bar", IsHost, "")

	ruleTest(t, "", IsDNSName, "")
	ruleTest(t, "-", IsDNSName, "invalid format")
	ruleTest(t, "foo.bar", IsDNSName, "")

	ruleTest(t, "", IsIPAddress, "")
	ruleTest(t, "-", IsIPAddress, "invalid format")
	ruleTest(t, "1.2.3.4", IsIPAddress, "")

	ruleTest(t, "", IsNumeric, "")
	ruleTest(t, "-", IsNumeric, "invalid format")
	ruleTest(t, "42", IsNumeric, "")

	ruleTest(t, "", IsValidUTF8, "")
	ruleTest(t, string([]byte{66, 250}), IsValidUTF8, "invalid format")
	ruleTest(t, "Ж", IsValidUTF8, "")

	ruleTest(t, "", IsVisible, "")
	ruleTest(t, " ", IsVisible, "invalid format")
	ruleTest(t, "foo", IsVisible, "")
}

func TestIsField(t *testing.T) {
	ruleTest(t, "Foo", IsField(&accessible{}, 1), "unknown field")
	ruleTest(t, "String", IsField(&accessible{}, 1), "invalid type")
	ruleTest(t, "String", IsField(&accessible{}, ""), "")
	ruleTest(t, "String", IsField(&accessible{}, 1, ""), "")
}

func BenchmarkValidate(b *testing.B) {
	i := 4
	str := "2"
	now := time.Now()
	obj := &validatable{
		Int:       1,
		Uint:      2,
		Float:     3,
		OptInt:    &i,
		String:    "1",
		OptString: &str,
		Strings:   []string{"3", "4"},
		Time:      time.Now(),
		OptTime:   &now,
	}
	for i := 0; i < b.N; i++ {
		err := Validate(obj, func(v *Validator) {
			v.Value("Int", false, IsMinInt(1))
			v.Value("Uint", false, IsMinUint(1))
			v.Value("Float", false, IsMinFloat(1))
			v.Value("OptInt", true, IsMinInt(1))
			v.Value("String", false, IsMinLen(1))
			v.Value("OptString", true, IsMinLen(1))
			v.Items("Strings", IsMinLen(1))
			v.Value("Time", false, IsNotZero)
			v.Value("OptTime", true, IsNotZero)
		})
		if err != nil {
			panic(err)
		}
	}
}
