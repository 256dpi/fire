package coal

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestRaw(t *testing.T) {
	type raw struct {
		Title string `json:"title"`
		Data  Raw    `json:"raw"`
	}

	input := &raw{
		Title: "foo",
		Data: MustRaw(map[string]interface{}{
			"bar": "baz",
		}),
	}

	jsonBytes, err := json.Marshal(input)
	assert.NoError(t, err)
	assert.Equal(t, `{"title":"foo","raw":{"bar":"baz"}}`, string(jsonBytes))

	var jsonOutput raw
	err = json.Unmarshal(jsonBytes, &jsonOutput)
	assert.NoError(t, err)
	assert.Equal(t, *input, jsonOutput)
	assert.Equal(t, "foo", jsonOutput.Title)

	jsonData, err := jsonOutput.Data.Get()
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"bar": "baz"}, jsonData)

	bsonBytes, err := bson.Marshal(input)
	assert.NoError(t, err)

	var bsonOutput raw
	err = bson.Unmarshal(bsonBytes, &bsonOutput)
	assert.NoError(t, err)
	assert.Equal(t, *input, bsonOutput)
	assert.Equal(t, "foo", bsonOutput.Title)

	bsonData, err := bsonOutput.Data.Get()
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"bar": "baz"}, bsonData)
}
