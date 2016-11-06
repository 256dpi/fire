package fire

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errAbortTest = errors.New("foo")

func TestPanic(t *testing.T) {
	var test error

	func() {
		defer Resume(func(err error) {
			test = err
		})

		Abort(errAbortTest)
	}()

	assert.Equal(t, errAbortTest, test)
}

func TestOtherPanic(t *testing.T) {
	defer func() {
		recover()
	}()

	var test error

	func() {
		defer Resume(func(err error) {
			test = err
		})

		panic(errAbortTest)
	}()

	assert.Nil(t, test)
}

func TestAssert(t *testing.T) {
	var test error

	func() {
		defer Resume(func(err error) {
			test = err
		})

		Assert(errAbortTest)
	}()

	assert.Equal(t, errAbortTest, test)
}

func TestNilAssert(t *testing.T) {
	var test error

	func() {
		defer Resume(func(err error) {
			test = err
		})

		Assert(nil)
	}()

	assert.Nil(t, test)
}
