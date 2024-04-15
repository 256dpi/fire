package coal

import (
	"reflect"

	"github.com/golang-sql/civil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
)

// Date defines a local date value.
type Date = civil.Date

// Time defines a local time value.
type Time = civil.Time

func init() {
	// register Date encoder and decoder
	var dateType = reflect.TypeOf(Date{})
	bson.DefaultRegistry.RegisterTypeEncoder(dateType, bsoncodec.ValueEncoderFunc(func(ec bsoncodec.EncodeContext, w bsonrw.ValueWriter, v reflect.Value) error {
		return w.WriteString(v.Interface().(Date).String())
	}))
	bson.DefaultRegistry.RegisterTypeDecoder(dateType, bsoncodec.ValueDecoderFunc(func(dc bsoncodec.DecodeContext, r bsonrw.ValueReader, v reflect.Value) error {
		str, err := r.ReadString()
		if err != nil {
			return err
		}
		if str == "0000-00-00" {
			v.Set(reflect.ValueOf(Date{}))
			return nil
		}
		date, err := civil.ParseDate(str)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(date))
		return nil
	}))

	// register Time encoder and decoder
	var timeType = reflect.TypeOf(Time{})
	bson.DefaultRegistry.RegisterTypeEncoder(timeType, bsoncodec.ValueEncoderFunc(func(ec bsoncodec.EncodeContext, w bsonrw.ValueWriter, v reflect.Value) error {
		return w.WriteString(v.Interface().(Time).String())
	}))
	bson.DefaultRegistry.RegisterTypeDecoder(timeType, bsoncodec.ValueDecoderFunc(func(dc bsoncodec.DecodeContext, r bsonrw.ValueReader, v reflect.Value) error {
		str, err := r.ReadString()
		if err != nil {
			return err
		}
		date, err := civil.ParseTime(str)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(date))
		return nil
	}))
}
