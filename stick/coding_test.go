package stick

import (
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
		name := GetJSONKey(&reflect.StructField{
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
		name := GetBSONKey(&reflect.StructField{
			Name: "Field",
			Tag:  reflect.StructTag(item.tag),
		})
		assert.Equal(t, item.name, name)
	}
}
