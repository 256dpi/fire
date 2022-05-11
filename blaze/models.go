package blaze

import (
	"mime"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson/bsontype"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

const maxFileNameLength = 256

// Link is used to link a file to a model.
type Link struct {
	// The user definable reference of the link.
	Ref string `json:"ref"`

	// The name of the linked files.
	Name string `json:"name" bson:"-"`

	// The media type of the linked file.
	Type string `json:"type" bson:"-"`

	// The size of the linked file.
	Size int64 `json:"size" bson:"-"`

	// The key for claiming a file.
	ClaimKey string `json:"claim-key" bson:"-"`

	// The key for viewing the linked file.
	ViewKey string `json:"view-key" bson:"-"`

	// The internal reference to the linked file.
	File coal.ID `json:"-" bson:"file_id"`

	// The internal information about the linked file.
	FileName string `json:"-" bson:"name"`
	FileType string `json:"-" bson:"type"`
	FileSize int64  `json:"-" bson:"size"`
}

// Validate will validate the link.
func (l *Link) Validate(requireFilename bool, whitelist ...string) error {
	// ensure reference
	if l.Ref == "" {
		l.Ref = coal.New().Hex()
	}

	return stick.Validate(l, func(v *stick.Validator) {
		v.Value("Ref", false, stick.IsNotZero)
		v.Value("File", false, stick.IsNotZero)
		if requireFilename {
			v.Value("FileName", false, stick.IsNotZero)
		}
		v.Value("FileType", false, func(stick.Subject) error {
			return ValidateType(l.FileType, whitelist...)
		})
		v.Value("FileSize", false, stick.IsMinInt(1))
	})
}

// IsValidLink returns a stick.Validate compatible link rule.
func IsValidLink(requireFilename bool, whitelist ...string) func(sub stick.Subject) error {
	return func(sub stick.Subject) error {
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

		// get link
		link, ok := sub.IValue.(Link)
		if !ok {
			panic("blaze: expected link value")
		}

		return link.Validate(requireFilename, whitelist...)
	}
}

// Links is a set of links.
type Links []Link

// UnmarshalBSONValue implements the bson.ValueUnmarshaler interface.
func (l *Links) UnmarshalBSONValue(typ bsontype.Type, bytes []byte) error {
	return stick.BSON.UnmarshalKeyedList(stick.InternalBSONValue(typ, bytes), l, "Ref")
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (l *Links) UnmarshalJSON(bytes []byte) error {
	return stick.JSON.UnmarshalKeyedList(bytes, l, "Ref")
}

// Validate will validate the links.
func (l Links) Validate(uniqueFilenames bool, whitelist ...string) error {
	// prepare maps
	refs := make(map[string]bool, len(l))
	names := make(map[string]bool, len(l))

	// validate all links
	for _, link := range l {
		// validate link
		err := link.Validate(uniqueFilenames, whitelist...)
		if err != nil {
			return err
		}

		// check ref uniqueness
		if refs[link.Ref] {
			return xo.SF("ambiguous reference")
		}

		// check filename uniqueness
		if uniqueFilenames && names[link.FileName] {
			return xo.SF("ambiguous filename")
		}

		// add reference
		refs[link.Ref] = true
		names[link.FileName] = true
	}

	return nil
}

// IsValidLinks returns a stick.Validate compatible links rule.
func IsValidLinks(uniqueFilenames bool, whitelist ...string) func(sub stick.Subject) error {
	return func(sub stick.Subject) error {
		// unwrap
		if !sub.Unwrap() {
			return nil
		}

		// get links
		links, ok := sub.IValue.(Links)
		if !ok {
			panic("blaze: expected links value")
		}

		return links.Validate(uniqueFilenames, whitelist...)
	}
}

// State describes the current state of a file. Usually, the state of a file
// always moves forward by one step, but it may also jump directly to "deleting".
type State string

// The individual states.
const (
	Uploading State = "uploading"
	Uploaded  State = "uploaded"
	Claimed   State = "claimed"
	Released  State = "released"
	Deleting  State = "deleting"
)

// Valid returns whether the state is valid.
func (s State) Valid() bool {
	switch s {
	case Uploading, Uploaded, Claimed, Released, Deleting:
		return true
	default:
		return false
	}
}

// File tracks uploaded files and their state.
type File struct {
	coal.Base `json:"-" bson:",inline" coal:"files"`

	// The current state of the file e.g. "uploading".
	State State `json:"state"`

	// The last time the file was updated.
	Updated time.Time `json:"updated-at" bson:"updated_at"`

	// The name of the file e.g. "logo.png".
	Name string `json:"name"`

	// The media type of the file e.g. "image/png".
	Type string `json:"type"`

	// The size of the file.
	Size int64 `json:"size"`

	// The service specific blob handle.
	Handle Handle `json:"handle"`

	// The binding of the file.
	Binding string `json:"binding"`

	// The owner of the file.
	Owner *coal.ID `json:"owner"`
}

// Validate will validate the model.
func (f *File) Validate() error {
	return stick.Validate(f, func(v *stick.Validator) {
		v.Value("State", false, stick.IsValid)
		v.Value("Updated", false, stick.IsNotZero)
		v.Value("Name", false, stick.IsMaxLen(maxFileNameLength))
		v.Value("Type", false, func(stick.Subject) error {
			return ValidateType(f.Type)
		})

		if f.State == Uploading {
			v.Value("Size", false, stick.IsMinInt(0))
		} else {
			v.Value("Size", false, stick.IsMinInt(1))
		}

		v.Value("Handle", false, stick.IsNotEmpty)

		if f.State == Claimed {
			v.Value("Binding", false, stick.IsNotZero)
			v.Value("Owner", false, stick.IsNotZero)
		} else {
			v.Value("Binding", false, stick.IsZero)
			v.Value("Owner", false, stick.IsZero)
		}
	})
}

// AddFileIndexes will add files indexes to the specified catalog.
func AddFileIndexes(catalog *coal.Catalog) {
	// index state for fast lookups
	catalog.AddIndex(&File{}, false, 0, "State", "Updated")
}

// ValidateType will validate a media type.
func ValidateType(str string, whitelist ...string) error {
	// check media type
	mediaType, _, err := mime.ParseMediaType(str)
	if err != nil {
		return xo.SF("type invalid")
	} else if str != mediaType {
		return xo.SF("type ambiguous")
	} else if len(whitelist) > 0 && !stick.Contains(whitelist, mediaType) {
		return xo.SF("type unallowed")
	}

	return nil
}
