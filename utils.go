package fire

import "go.mongodb.org/mongo-driver/bson/primitive"

func isValidObjectID(str string) bool {
	_, err := primitive.ObjectIDFromHex(str)
	return err == nil
}

func mustObjectIDFromHex(str string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(str)
	if err != nil {
		panic(err)
	}

	return id
}
