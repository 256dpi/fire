// Package coal provides a mini ORM for mongoDB.
package coal

import (
	"reflect"
	"time"

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

// Item is the base for every coal model item.
type Item struct {
	ItemID ID `json:"_id,omitempty" bson:"_id,omitempty"`
}

// I is a shorthand to construct an item with the provided id or a generated
// id if none specified.
func I(id ...ID) Item {
	// check list
	if len(id) > 1 {
		panic("coal: I accepts only one id")
	}

	// use provided id id available
	if len(id) > 0 {
		return Item{
			ItemID: id[0],
		}
	}

	return Item{
		ItemID: New(),
	}
}

// GetAccessor implements the Model interface.
func (i *Item) GetAccessor(v interface{}) *stick.Accessor {
	return GetItemMeta(reflect.TypeOf(v)).Accessor
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
