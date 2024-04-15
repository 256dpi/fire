package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

type dateTest struct {
	V Date
	P *Date
	L []Date
	M map[string]Date
}

type timeTest struct {
	V Time
	P *Time
	L []Time
	M map[string]Time
}

func TestDateCoding(t *testing.T) {
	bytes, err := bson.Marshal(dateTest{})
	assert.NoError(t, err)

	var m bson.M
	err = bson.Unmarshal(bytes, &m)
	assert.NoError(t, err)
	assert.Equal(t, bson.M{
		"v": "0000-00-00",
		"p": nil,
		"l": nil,
		"m": nil,
	}, m)

	var out dateTest
	err = bson.Unmarshal(bytes, &out)
	assert.NoError(t, err)
	assert.Equal(t, dateTest{}, out)

	bytes, err = bson.Marshal(dateTest{
		V: Date{2024, 4, 15},
		P: &Date{2024, 4, 15},
		L: []Date{{2024, 4, 15}},
		M: map[string]Date{
			"date": {2024, 4, 15},
		},
	})
	assert.NoError(t, err)

	m = bson.M{}
	err = bson.Unmarshal(bytes, &m)
	assert.NoError(t, err)
	assert.Equal(t, bson.M{
		"v": "2024-04-15",
		"p": "2024-04-15",
		"l": bson.A{"2024-04-15"},
		"m": bson.M{
			"date": "2024-04-15",
		},
	}, m)

	out = dateTest{}
	err = bson.Unmarshal(bytes, &out)
	assert.NoError(t, err)
	assert.Equal(t, dateTest{
		V: Date{2024, 4, 15},
		P: &Date{2024, 4, 15},
		L: []Date{{2024, 4, 15}},
		M: map[string]Date{
			"date": {2024, 4, 15},
		},
	}, out)
}

func TestTimeCoding(t *testing.T) {
	bytes, err := bson.Marshal(timeTest{})
	assert.NoError(t, err)

	var m bson.M
	err = bson.Unmarshal(bytes, &m)
	assert.NoError(t, err)
	assert.Equal(t, bson.M{
		"v": "00:00:00",
		"p": nil,
		"l": nil,
		"m": nil,
	}, m)

	var out timeTest
	err = bson.Unmarshal(bytes, &out)
	assert.NoError(t, err)
	assert.Equal(t, timeTest{}, out)

	bytes, err = bson.Marshal(timeTest{
		V: Time{12, 6, 3, 1},
		P: &Time{12, 6, 3, 1},
		L: []Time{{12, 6, 3, 1}},
		M: map[string]Time{
			"noon": {12, 6, 3, 1},
		},
	})
	assert.NoError(t, err)

	m = bson.M{}
	err = bson.Unmarshal(bytes, &m)
	assert.NoError(t, err)
	assert.Equal(t, bson.M{
		"v": "12:06:03.000000001",
		"p": "12:06:03.000000001",
		"l": bson.A{"12:06:03.000000001"},
		"m": bson.M{
			"noon": "12:06:03.000000001",
		},
	}, m)

	out = timeTest{}
	err = bson.Unmarshal(bytes, &out)
	assert.NoError(t, err)
	assert.Equal(t, timeTest{
		V: Time{12, 6, 3, 1},
		P: &Time{12, 6, 3, 1},
		L: []Time{{12, 6, 3, 1}},
		M: map[string]Time{
			"noon": {12, 6, 3, 1},
		},
	}, out)
}
