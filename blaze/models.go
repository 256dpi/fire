package blaze

import (
	"fmt"
	"mime"
	"time"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Link is used to link a file to a model.
type Link struct {
	// The media type of the linked file.
	Type string `json:"type" bson:"-"`

	// The size of the linked file.
	Size int64 `json:"size" bson:"-"`

	// The key for claiming a file.
	ClaimKey string `json:"claim-key" bson:"-"`

	// The key for viewing the linked file.
	ViewKey string `json:"view-key" bson:"-"`

	// The internal reference to the linked file.
	File *coal.ID `json:"-" bson:"file_id"`

	// The internal information about the linked file.
	FileType string `json:"-" bson:"type"`
	FileSize int64  `json:"-" bson:"size"`
}

// Validate will validate the link.
func (l *Link) Validate(name string, whitelist ...string) error {
	// check file
	if l.File == nil || l.File.IsZero() {
		return stick.E("%s invalid file", name)
	}

	// check file type
	err := ValidateType(l.FileType, whitelist...)
	if err != nil {
		return stick.E("%s %s", name, err.Error())
	}

	// check file size
	if l.FileSize <= 0 {
		return stick.E("%s zero size", name)
	}

	return nil
}

// State describes the current state of a file. Usually, the state of a file
// always moves forward by one step but it may also jump directly to "deleting".
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
	// check state
	if !f.State.Valid() {
		return fmt.Errorf("invalid state")
	}

	// check type
	err := ValidateType(f.Type)
	if err != nil {
		return stick.E("type %s", err.Error())
	}

	// check size
	if f.Size <= 0 && (f.State == Uploaded || f.State == Claimed) {
		return stick.E("missing size")
	}

	// check handle
	if len(f.Handle) == 0 {
		return stick.E("missing handle")
	}

	// check binding
	if f.Binding == "" && f.State == Claimed {
		return stick.E("missing binding")
	}

	// check owner
	if f.Owner != nil && f.Owner.IsZero() {
		return stick.E("invalid owner")
	} else if f.Owner == nil && f.State == Claimed {
		return stick.E("missing owner")
	}

	return nil
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
		return stick.E("type invalid")
	} else if str != mediaType {
		return stick.E("type ambiguous")
	} else if len(whitelist) > 0 && !stick.Contains(whitelist, mediaType) {
		return stick.E("type unallowed")
	}

	return nil
}
