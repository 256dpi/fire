package stick

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestMap(t *testing.T) {
	type child struct {
		Body string `json:"the-body"`
	}

	type parent struct {
		Title string `json:"title"`
		Data  Map    `json:"data"`
	}

	input := &parent{
		Title: "foo",
		Data:  MustMap(child{Body: "body"}, JSON),
	}

	bytes1, err := json.Marshal(input)
	assert.NoError(t, err)
	assert.Equal(t, `{"title":"foo","data":{"the-body":"body"}}`, string(bytes1))

	var output1 parent
	err = json.Unmarshal(bytes1, &output1)
	assert.NoError(t, err)
	assert.Equal(t, parent{
		Title: "foo",
		Data: Map{
			"the-body": "body",
		},
	}, output1)

	var ch1 child
	output1.Data.MustUnmarshal(&ch1, JSON)
	assert.Equal(t, child{Body: "body"}, ch1)

	bytes2, err := bson.Marshal(input)
	assert.NoError(t, err)

	var output2 parent
	err = bson.Unmarshal(bytes2, &output2)
	assert.NoError(t, err)
	assert.Equal(t, parent{
		Title: "foo",
		Data: Map{
			"the-body": "body",
		},
	}, output2)

	var ch2 child
	output2.Data.MustUnmarshal(&ch2, JSON)
	assert.Equal(t, child{Body: "body"}, ch2)
}

func TestMapFlat(t *testing.T) {
	m := Map{
		"foo": "bar",
		"bar": Map{
			"foo": "bar",
		},
	}

	assert.Equal(t, Map{
		"foo":     "bar",
		"bar_foo": "bar",
	}, m.Flat("_"))
}
