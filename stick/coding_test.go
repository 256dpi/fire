package stick

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	return JSON.UnmarshalKeyedList(bytes, t, "id")
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
		{"id": "a"}
	]`), &list)
	assert.NoError(t, err)
	assert.Equal(t, testListMap{
		{"id": "c", "val": 4.0},
		{"id": "d", "val": 5.0},
		{"id": "a", "val": 1.0},
	}, list)
}

type testStruct struct {
	ID  string `json:"id"`
	Val int64  `json:"val"`
}

type testListStruct []testStruct

func (t *testListStruct) UnmarshalJSON(bytes []byte) error {
	return JSON.UnmarshalKeyedList(bytes, t, "ID")
}

func TestCodingUnmarshalKeyedListStruct(t *testing.T) {
	list := testListStruct{
		{ID: "a", Val: 1},
		{ID: "b", Val: 2},
		{ID: "c", Val: 3},
	}

	err := json.Unmarshal([]byte(`[
		{"id": "c", "val": 4},
		{"id": "d", "val": 5},
		{"id": "a"}
	]`), &list)
	assert.NoError(t, err)
	assert.Equal(t, testListStruct{
		{ID: "c", Val: 4},
		{ID: "d", Val: 5},
		{ID: "a", Val: 1},
	}, list)
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
