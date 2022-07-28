package glut

import (
	"time"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire/stick"
)

// GetKey will get the key of a value.
func GetKey(value Value) (string, error) {
	// get meta
	meta := GetMeta(value)

	// prepare key
	key := meta.Key

	// check if extended
	ev, ok := value.(ExtendedValue)
	if !ok {
		return key, nil
	}

	// validate value
	err := value.Validate()
	if err != nil {
		return "", err
	}

	// get extension
	extension := ev.GetExtension()

	// check extension
	if extension == "" {
		return "", xo.F("missing extension")
	}

	return key + extension, nil
}

// GetDeadline will get the deadline of a value.
func GetDeadline(value Value) (*time.Time, error) {
	// get meta
	meta := GetMeta(value)

	// prepare deadline
	var deadline *time.Time
	if meta.TTL > 0 {
		deadline = stick.P(time.Now().Add(meta.TTL))
	}

	// check if restricted
	rv, ok := value.(RestrictedValue)
	if !ok {
		return deadline, nil
	}

	// validate value
	err := value.Validate()
	if err != nil {
		return nil, err
	}

	// get deadline
	newDeadline := rv.GetDeadline()

	// return if unavailable
	if newDeadline == nil {
		return deadline, nil
	}

	// check deadline
	if newDeadline != nil && newDeadline.IsZero() {
		return nil, xo.F("zero deadline")
	}

	return newDeadline, nil
}
