// Package coal provides a mini ORM for mongoDB.
package coal

import (
	"reflect"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson/bsontype"

	"github.com/256dpi/fire/stick"
)

// Tag is underlying model tag structure.
type Tag struct {
	Value  interface{} `bson:"v"`
	Expiry time.Time   `bson:"e,omitempty"`
}

// Model defines the shape of a document stored in a collection. Custom types
// must implement the interface by embedding the Base type.
type Model interface {
	ID() ID
	Validate() error
	GetBase() *Base
	GetAccessor(interface{}) *stick.Accessor
}

// Base is the base for every coal model.
type Base struct {
	DocID ID             `json:"-" bson:"_id,omitempty"`
	Lock  int64          `json:"-" bson:"_lk,omitempty"`
	Token ID             `json:"-" bson:"_tk,omitempty"`
	Score float64        `json:"-" bson:"_sc,omitempty"`
	Tags  map[string]Tag `json:"-" bson:"_tg,omitempty"`
}

// B is a shorthand to construct a base with the provided id or a generated
// id if none specified.
func B(id ...ID) Base {
	// check list
	if len(id) > 1 {
		panic("coal: B accepts only one id")
	}

	// use provided id id available
	if len(id) > 0 {
		return Base{
			DocID: id[0],
		}
	}

	return Base{
		DocID: New(),
	}
}

// ID implements the Model interface.
func (b *Base) ID() ID {
	return b.DocID
}

// GetBase implements the Model interface.
func (b *Base) GetBase() *Base {
	return b
}

// GetTag will get the value for the specified tag.
func (b *Base) GetTag(name string) interface{} {
	tv, ok := b.Tags[name]
	if ok && (tv.Expiry.IsZero() || tv.Expiry.After(time.Now())) {
		return tv.Value
	}
	return nil
}

// SetTag will set the provided value for the specified tag.
func (b *Base) SetTag(name string, value interface{}, expiry time.Time) {
	if b.Tags == nil {
		b.Tags = map[string]Tag{}
	}
	if value == nil {
		delete(b.Tags, name)
		return
	}
	b.Tags[name] = Tag{
		Value:  value,
		Expiry: expiry,
	}
}

// GetAccessor implements the Model interface.
func (b *Base) GetAccessor(v interface{}) *stick.Accessor {
	return GetMeta(v.(Model)).Accessor
}

// Item defines the shape of an item stored in a model. Custom types
// must implement the interface by embedding the Base type.
type Item interface {
	ID() string
	Validate() error
	GetBase() *ItemBase
	GetAccessor(interface{}) *stick.Accessor
}

// ItemBase is the base for every coal model item.
type ItemBase struct {
	ItemID string `json:"id,omitempty" bson:"_id,omitempty"`
}

// I is a shorthand to construct an item with the provided id or a generated
// id if none specified.
func I(id ...string) ItemBase {
	// check list
	if len(id) > 1 {
		panic("coal: I accepts only one id")
	}

	// use provided id id available
	if len(id) > 0 {
		return ItemBase{
			ItemID: id[0],
		}
	}

	return ItemBase{
		ItemID: New().Hex(),
	}
}

// ID implements the Item interface.
func (b *ItemBase) ID() string {
	return b.ItemID
}

// GetBase implements the Item interface.
func (b *ItemBase) GetBase() *ItemBase {
	return b
}

// GetAccessor implements the Model interface.
func (*ItemBase) GetAccessor(v interface{}) *stick.Accessor {
	return GetItemMeta(reflect.TypeOf(v)).Accessor
}

// List wraps any type that embeds ItemBase as a slice that automatically merges
// existing items with new items if they have the same ID values.
type List[T Item] []T

// Validate will validate all items and return the first error.
func (l *List[T]) Validate() error {
	// check value
	for _, entry := range *l {
		if reflect.ValueOf(entry).IsNil() {
			return xo.SF("nil item")
		}
	}

	// ensure IDs
	for _, entry := range *l {
		base := entry.GetBase()
		if base.ItemID == "" {
			base.ItemID = New().Hex()
		}
	}

	// validate items
	for _, item := range *l {
		err := item.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalJSON implement the json.Unmarshaler interface.
func (l *List[T]) UnmarshalJSON(bytes []byte) error {
	return stick.UnmarshalKeyedList(stick.JSON, bytes, l, func(item T) string {
		return item.ID()
	})
}

// UnmarshalBSONValue implement the bson.ValueUnmarshaler interface.
func (l *List[T]) UnmarshalBSONValue(typ bsontype.Type, bytes []byte) error {
	return stick.UnmarshalKeyedList(stick.BSON, stick.InternalBSONValue(typ, bytes), l, func(item T) string {
		return item.ID()
	})
}

// Clean will clean the model by removing expired tags.
func Clean(model Model) {
	// deleted expired tags
	tags := model.GetBase().Tags
	now := time.Now()
	for name, tag := range tags {
		if !tag.Expiry.IsZero() && tag.Expiry.Before(now) {
			delete(tags, name)
		}
	}
}

// Slice takes a slice of the form []Post, []*Post, *[]Post or *[]*Post and
// returns a new slice that contains all models.
func Slice(val interface{}) []Model {
	// get slice
	slice := reflect.ValueOf(val)
	if slice.Kind() == reflect.Ptr {
		slice = slice.Elem()
	}

	// collect models
	models := make([]Model, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		model := slice.Index(i)
		if model.Kind() == reflect.Struct {
			model = model.Addr()
		}
		models[i] = model.Interface().(Model)
	}

	return models
}

// Registry is a collection of known models.
type Registry struct {
	*stick.Registry[Model]
}

// NewRegistry will return a model registry indexed by plural name.
func NewRegistry(models ...Model) *Registry {
	return &Registry{
		Registry: stick.NewRegistry(models,
			nil,
			// index by plural name
			func(model Model) string {
				return GetMeta(model).PluralName
			},
		),
	}
}

// Lookup will lookup a model by its plural name.
func (r *Registry) Lookup(pluralName string) Model {
	// lookup model
	model, _ := r.Registry.Lookup(0, pluralName)

	return model
}
