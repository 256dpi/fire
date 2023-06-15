package coal

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
)

// Extension is a function that registers custom encoder and decoder on the
// provided BSON registry builder.
type Extension func(builder *bsoncodec.RegistryBuilder)

var extensions []Extension

// Extend will register the provided extension and build and set the default
// BSON registry.
func Extend(ext Extension) {
	// add extensions
	extensions = append(extensions, ext)

	// create builder
	builder := bson.NewRegistryBuilder()

	// run extensions
	for _, ext := range extensions {
		ext(builder)
	}

	// build and replace registry
	bson.DefaultRegistry = builder.Build()
}
