package coal

import (
	"errors"
	"reflect"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Decimal supports coding to and from the BSON decimal128 type.
type Decimal = decimal.Decimal

var decimalType = reflect.TypeOf(Decimal{})

func init() {
	Extend(func(builder *bsoncodec.RegistryBuilder) {
		// register decimal encoder
		var dve = bsoncodec.DefaultValueEncoders{}
		builder.RegisterTypeEncoder(decimalType, bsoncodec.ValueEncoderFunc(func(ec bsoncodec.EncodeContext, w bsonrw.ValueWriter, v reflect.Value) error {
			// convert value
			dec := v.Interface().(Decimal)
			pd, ok := primitive.ParseDecimal128FromBigInt(dec.Coefficient(), int(dec.Exponent()))
			if !ok {
				return errors.New("unable to convert decimal value")
			}

			// encode value
			err := dve.Decimal128EncodeValue(ec, w, reflect.ValueOf(pd))
			if err != nil {
				return err
			}

			return nil
		}))

		// register decimal decoder
		var dvd = bsoncodec.DefaultValueDecoders{}
		builder.RegisterTypeDecoder(decimalType, bsoncodec.ValueDecoderFunc(func(dc bsoncodec.DecodeContext, r bsonrw.ValueReader, v reflect.Value) error {
			// decode value
			val := reflect.New(reflect.TypeOf(primitive.Decimal128{})).Elem()
			err := dvd.Decimal128DecodeValue(dc, r, val)
			if err != nil {
				return err
			}

			// get value
			pd := val.Interface().(primitive.Decimal128)
			big, exp, err := pd.BigInt()
			if err != nil {
				return err
			}

			// set value
			v.Set(reflect.ValueOf(decimal.NewFromBigInt(big, int32(exp))))

			return nil
		}))
	})
}
