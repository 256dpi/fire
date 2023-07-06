package stick

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

var testCodings = []Coding{JSON, BSON}

func TestCoding(t *testing.T) {
	for _, coding := range testCodings {
		/* primitive */

		str, err := coding.Marshal("Hello world!")
		assert.NoError(t, err)
		assert.NotEmpty(t, str)

		var wrongStr int
		err = coding.Unmarshal(str, &wrongStr)
		assert.Error(t, err)

		var correctStr string
		err = coding.Unmarshal(str, &correctStr)
		assert.NoError(t, err)
		assert.Equal(t, "Hello world!", correctStr)

		/* list */

		list, err := coding.Marshal([]interface{}{"1", true, 2.2})
		assert.NoError(t, err)
		assert.NotEmpty(t, list)

		var wrongList map[string]interface{}
		err = coding.Unmarshal(list, &wrongList)
		assert.Error(t, err)

		var correctList []interface{}
		err = coding.Unmarshal(list, &correctList)
		assert.NoError(t, err)
		assert.Equal(t, []interface{}{"1", true, 2.2}, correctList)

		/* map */

		object, err := coding.Marshal(map[string]interface{}{
			"a": "1",
			"b": true,
			"c": 2.2,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, object)

		var wrongObject []interface{}
		err = coding.Unmarshal(object, &wrongObject)
		assert.Error(t, err)

		var correctObject map[string]interface{}
		err = coding.Unmarshal(object, &correctObject)
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"a": "1",
			"b": true,
			"c": 2.2,
		}, correctObject)
	}
}

func TestCodingTransfer(t *testing.T) {
	for _, coding := range testCodings {
		inObj := map[string]interface{}{
			"foo": "bar",
		}
		var outObj map[string]interface{}
		err := coding.Transfer(inObj, &outObj)
		assert.NoError(t, err)
		assert.Equal(t, inObj, outObj)

		inArr := []interface{}{"foo"}
		var outArr []interface{}
		err = coding.Transfer(inArr, &outArr)
		assert.NoError(t, err)
		assert.Equal(t, inArr, outArr)
	}
}

func TestParseJSONTag(t *testing.T) {
	table := []struct {
		tag  string
		name string
	}{
		{
			tag:  "",
			name: "Field",
		},
		{
			tag:  `json:""`,
			name: "Field",
		},
		{
			tag:  `json:"-"`,
			name: "",
		},
		{
			tag:  `json:","`,
			name: "Field",
		},
		{
			tag:  `json:"-,"`,
			name: "-",
		},
		{
			tag:  `json:"field"`,
			name: "field",
		},
		{
			tag:  `json:"field,"`,
			name: "field",
		},
	}

	for _, item := range table {
		name := JSON.GetKey(reflect.StructField{
			Name: "Field",
			Tag:  reflect.StructTag(item.tag),
		})
		assert.Equal(t, item.name, name)
	}
}

func TestParseBSONTag(t *testing.T) {
	table := []struct {
		tag  string
		name string
	}{
		{
			tag:  "",
			name: "field",
		},
		{
			tag:  `bson:""`,
			name: "field",
		},
		{
			tag:  `bson:"-"`,
			name: "",
		},
		{
			tag:  `bson:","`,
			name: "field",
		},
		{
			tag:  `bson:"-,"`,
			name: "-",
		},
		{
			tag:  `bson:"Field"`,
			name: "Field",
		},
		{
			tag:  `bson:"Field,"`,
			name: "Field",
		},
	}

	for _, item := range table {
		name := BSON.GetKey(reflect.StructField{
			Name: "Field",
			Tag:  reflect.StructTag(item.tag),
		})
		assert.Equal(t, item.name, name)
	}
}

type testListMap []Map

func (t *testListMap) UnmarshalJSON(bytes []byte) error {
	return UnmarshalKeyedList(JSON, bytes, t, func(t Map) string {
		id, _ := t["id"].(string)
		return id
	})
}

func TestCodingUnmarshalKeyedListMap(t *testing.T) {
	list := testListMap{
		{"id": "a", "val": 1.0},
		{"id": "b", "val": 2.0},
		{"id": "c", "val": 3.0},
	}

	err := json.Unmarshal([]byte(`[
		{"id": "c", "val": 4.0},
		{"id": "d", "val": 5.0},
		{"id": "a"},
		{"val": 6.0}
	]`), &list)
	assert.NoError(t, err)
	assert.Equal(t, testListMap{
		{"id": "c", "val": 4.0},
		{"id": "d", "val": 5.0},
		{"id": "a", "val": 1.0},
		{"val": 6.0},
	}, list)
}

type testStruct struct {
	ID  string `json:"id"`
	Val int64  `json:"val"`
}

type testListStruct []testStruct

func (t *testListStruct) UnmarshalJSON(bytes []byte) error {
	return UnmarshalKeyedList(JSON, bytes, t, func(t testStruct) string {
		return t.ID
	})
}

func (t *testListStruct) UnmarshalBSONValue(typ bsontype.Type, bytes []byte) error {
	return UnmarshalKeyedList(BSON, InternalBSONValue(typ, bytes), t, func(t testStruct) string {
		return t.ID
	})
}

func TestCodingUnmarshalKeyedListStructJSON(t *testing.T) {
	list := testListStruct{
		{ID: "a", Val: 1},
		{ID: "b", Val: 2},
		{ID: "c", Val: 3},
	}

	err := json.Unmarshal([]byte(`[
		{"id": "c", "val": 4},
		{"id": "d", "val": 5},
		{"id": "a"},
		{"val": 6}
	]`), &list)
	assert.NoError(t, err)
	assert.Equal(t, testListStruct{
		{ID: "c", Val: 4},
		{ID: "d", Val: 5},
		{ID: "a", Val: 1},
		{Val: 6},
	}, list)

	err = json.Unmarshal([]byte(`[]`), &list)
	assert.NoError(t, err)
	assert.Equal(t, testListStruct{}, list)

	err = json.Unmarshal([]byte(`null`), &list)
	assert.NoError(t, err)
	assert.Equal(t, testListStruct(nil), list)
}

func TestCodingUnmarshalKeyedListStructBSON(t *testing.T) {
	list := testListStruct{
		{ID: "a", Val: 1},
		{ID: "b", Val: 2},
		{ID: "c", Val: 3},
	}

	err := BSON.Unmarshal(asBSON(bson.A{
		bson.M{"id": "c", "val": 4},
		bson.M{"id": "d", "val": 5},
		bson.M{"id": "a"},
		bson.M{"val": 6},
	}), &list)
	assert.NoError(t, err)
	assert.Equal(t, testListStruct{
		{ID: "c", Val: 4},
		{ID: "d", Val: 5},
		{ID: "a", Val: 1},
		{Val: 6},
	}, list)

	err = BSON.Unmarshal(asBSON(bson.A{}), &list)
	assert.NoError(t, err)
	assert.Equal(t, testListStruct{}, list)

	err = BSON.Unmarshal(asBSON(nil), &list)
	assert.NoError(t, err)
	assert.Equal(t, testListStruct(nil), list)
}

func BenchmarkCodingUnmarshalKeyedListStruct(b *testing.B) {
	list := testListStruct{
		{ID: "a", Val: 1},
		{ID: "b", Val: 2},
		{ID: "c", Val: 3},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := json.Unmarshal([]byte(`[
		{"id": "c", "val": 4},
		{"id": "d", "val": 5},
		{"id": "a"}
	]`), &list)
		if err != nil {
			panic(err)
		}
	}
}

func asBSON(v interface{}) []byte {
	bytes, err := BSON.Marshal(v)
	if err != nil {
		panic(err)
	}

	return bytes
}
