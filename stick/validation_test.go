package stick

import (
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

type validatable struct {
	Int       int32
	Uint      uint16
	Float     float64
	OptInt    *int
	String    string
	OptString *string //
	Strings   []string
	Time      time.Time  // never zero
	OptTime   *time.Time // never zero
	BasicAccess
}

func (v validatable) Validate() error {
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

	obj.Strings = []string{""}
	err = Validate(obj, func(v *Validator) {
		v.Value("Strings", false, IsMinLen(5))
		v.Items("Strings", IsMinLen(5))
	})
	assert.Error(t, err)
	assert.Equal(t, "Strings: too short; Strings.0: too short", err.Error())
}

func ruleTest(t *testing.T, val interface{}, rule Rule, msg string) {
	ctx := RuleContext{
		IValue: val,
		RValue: reflect.ValueOf(val),
	}

	err := rule(ctx)
	if msg == "" {
		assert.NoError(t, err)
	} else {
		assert.Error(t, err)
		assert.Equal(t, msg, err.Error())
	}
}

func TestUser(t *testing.T) {
	ruleTest(t, 1, Use(func() error {
		return nil
	}), "")

	ruleTest(t, 1, Use(func() error {
		return xo.SF("foo")
	}), "foo")
}

type zeroStr string

func (s zeroStr) Zero() bool {
	return s == "zero"
}

func TestIsZero(t *testing.T) {
	foo := "foo"
	empty := ""
	ruleTest(t, foo, IsZero, "not zero")
	ruleTest(t, empty, IsZero, "")
	ruleTest(t, nil, IsZero, "")
	ruleTest(t, &foo, IsZero, "not zero")
	ruleTest(t, &empty, IsZero, "not zero")

	now := time.Now()
	var nilTime *time.Time
	ruleTest(t, now, IsZero, "not zero")
	ruleTest(t, &now, IsZero, "not zero")
	ruleTest(t, nilTime, IsZero, "")
	ruleTest(t, time.Time{}, IsZero, "")
	ruleTest(t, &time.Time{}, IsZero, "")

	zero := zeroStr("zero")
	ruleTest(t, zero, IsZero, "")
	ruleTest(t, &zero, IsZero, "")
	ruleTest(t, zeroStr(""), IsZero, "not zero")
}

func TestIsNotZero(t *testing.T) {
	foo := "foo"
	empty := ""
	ruleTest(t, foo, IsNotZero, "")
	ruleTest(t, empty, IsNotZero, "zero")
	ruleTest(t, nil, IsNotZero, "zero")
	ruleTest(t, &foo, IsNotZero, "")
	ruleTest(t, &empty, IsNotZero, "")

	now := time.Now()
	var nilTime *time.Time
	ruleTest(t, now, IsNotZero, "")
	ruleTest(t, &now, IsNotZero, "")
	ruleTest(t, nilTime, IsNotZero, "zero")
	ruleTest(t, time.Time{}, IsNotZero, "zero")
	ruleTest(t, &time.Time{}, IsNotZero, "zero")

	zero := zeroStr("zero")
	ruleTest(t, zero, IsNotZero, "zero")
	ruleTest(t, &zero, IsNotZero, "zero")
	ruleTest(t, zeroStr(""), IsNotZero, "")
}

func TestNotEmpty(t *testing.T) {
	ruleTest(t, "", IsNotEmpty, "empty")
	ruleTest(t, "foo", IsNotEmpty, "")

	ruleTest(t, [0]int{}, IsNotEmpty, "empty")
	ruleTest(t, [1]int{1}, IsNotEmpty, "")

	ruleTest(t, []byte{}, IsNotEmpty, "empty")
	ruleTest(t, []byte{1}, IsNotEmpty, "")

	ruleTest(t, Map{}, IsNotEmpty, "empty")
	ruleTest(t, Map{"k": "v"}, IsNotEmpty, "")
}

type validStr string

func (s validStr) Valid() bool {
	return s == "valid"
}

type isValidStr string

func (s isValidStr) IsValid() bool {
	return s == "valid"
}

func TestIsValid(t *testing.T) {
	assert.PanicsWithValue(t, `stick: cannot check validity of string`, func() {
		ruleTest(t, "", IsValid, "")
	})

	ruleTest(t, validatable{}, IsValid, "invalid")
	ruleTest(t, &validatable{String: "valid"}, IsValid, "")

	ruleTest(t, validStr(""), IsValid, "invalid")
	ruleTest(t, validStr("valid"), IsValid, "")

	ruleTest(t, isValidStr(""), IsValid, "invalid")
	ruleTest(t, isValidStr("valid"), IsValid, "")
}

func TestIsMinLen(t *testing.T) {
	ruleTest(t, "", IsMinLen(5), "too short")
	ruleTest(t, "Hello World!", IsMinLen(5), "")
}

func TestIsMaxLen(t *testing.T) {
	ruleTest(t, "", IsMaxLen(5), "")
	ruleTest(t, "Hello World!", IsMaxLen(5), "too long")
}

func TestIsMin(t *testing.T) {
	assert.PanicsWithValue(t, "stick: expected int value", func() {
		ruleTest(t, uint(1), IsMinInt(1), "")
	})
	ruleTest(t, 7, IsMinInt(5), "")
	ruleTest(t, int16(1), IsMinInt(5), "too small")

	assert.PanicsWithValue(t, "stick: expected uint value", func() {
		ruleTest(t, 1, IsMinUint(1), "")
	})
	ruleTest(t, uint(7), IsMinUint(5), "")
	ruleTest(t, uint16(1), IsMinUint(5), "too small")

	assert.PanicsWithValue(t, "stick: expected float value", func() {
		ruleTest(t, 1, IsMinFloat(1), "")
	})
	ruleTest(t, 7., IsMinFloat(5), "")
	ruleTest(t, float32(1), IsMinFloat(5), "too small")
}

func TestIsMax(t *testing.T) {
	assert.PanicsWithValue(t, "stick: expected int value", func() {
		ruleTest(t, uint(1), IsMaxInt(1), "")
	})
	ruleTest(t, 1, IsMaxInt(5), "")
	ruleTest(t, int16(7), IsMaxInt(5), "too big")

	assert.PanicsWithValue(t, "stick: expected uint value", func() {
		ruleTest(t, 1, IsMaxUint(1), "")
	})
	ruleTest(t, uint(1), IsMaxUint(5), "")
	ruleTest(t, uint16(7), IsMaxUint(5), "too big")

	assert.PanicsWithValue(t, "stick: expected float value", func() {
		ruleTest(t, 1, IsMaxFloat(1), "")
	})
	ruleTest(t, 1., IsMaxFloat(5), "")
	ruleTest(t, float32(7), IsMaxFloat(5), "too big")
}

func TestIsFormat(t *testing.T) {
	assert.PanicsWithValue(t, `stick: expected string value`, func() {
		ruleTest(t, 1, IsEmail, "")
	})

	ruleTest(t, "", IsPatternMatch("\\d+"), "")
	ruleTest(t, "-", IsPatternMatch("\\d+"), "invalid format")
	ruleTest(t, "7", IsPatternMatch("\\d+"), "")

	ruleTest(t, "", IsEmail, "")
	ruleTest(t, "-", IsEmail, "invalid format")
	ruleTest(t, "foo@bar.com", IsEmail, "")

	ruleTest(t, "", IsURL, "")
	ruleTest(t, "-", IsURL, "invalid format")
	ruleTest(t, "foo.bar/baz", IsURL, "")

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
}

func TestRuleContextGuard(t *testing.T) {
	i1 := 1
	ruleTest(t, &i1, IsMaxInt(5), "")

	var i2 *int
	ruleTest(t, i2, IsMaxInt(5), "")
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
