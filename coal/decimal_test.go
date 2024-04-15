package coal

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var decimalPI = decimal.NewFromFloat(3.14159265)
var primitivePI, _ = primitive.ParseDecimal128("3.14159265")
var primitiveZero, _ = primitive.ParseDecimal128("0")

type decTest struct {
	D Decimal
	P *Decimal
	L []Decimal
	M map[string]Decimal
}

func TestDecimalCoding(t *testing.T) {
	bytes, err := bson.Marshal(decTest{})
	assert.NoError(t, err)

	var m bson.M
	err = bson.Unmarshal(bytes, &m)
	assert.NoError(t, err)
	assert.Equal(t, bson.M{
		"d": primitiveZero,
		"p": nil,
		"l": nil,
		"m": nil,
	}, m)

	var out decTest
	err = bson.Unmarshal(bytes, &out)
	assert.NoError(t, err)
	assert.Equal(t, decTest{
		D: decimal.New(0, 0),
	}, out)

	bytes, err = bson.Marshal(decTest{
		D: decimalPI,
		P: &decimalPI,
		L: []Decimal{decimalPI},
		M: map[string]Decimal{
			"pi": decimalPI,
		},
	})
	assert.NoError(t, err)

	m = bson.M{}
	err = bson.Unmarshal(bytes, &m)
	assert.NoError(t, err)
	assert.Equal(t, bson.M{
		"d": primitivePI,
		"p": primitivePI,
		"l": bson.A{primitivePI},
		"m": bson.M{
			"pi": primitivePI,
		},
	}, m)

	out = decTest{}
	err = bson.Unmarshal(bytes, &out)
	assert.NoError(t, err)
	assert.Equal(t, decTest{
		D: decimalPI,
		P: &decimalPI,
		L: []Decimal{decimalPI},
		M: map[string]Decimal{
			"pi": decimalPI,
		},
	}, out)
}
