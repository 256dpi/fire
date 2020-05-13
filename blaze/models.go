package blaze

import (
	"fmt"
	"mime"
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Link is used to link a file to a model.
type Link struct {
	// The type of the linked file.
	Type string `json:"type" bson:"-"`

	// The length of the linked file.
	Length int64 `json:"length" bson:"-"`

	// The key for claiming a file.
	ClaimKey string `json:"claim-key" bson:"-"`

	// The key for viewing the linked file.
	ViewKey string `json:"view-key" bson:"-"`

	// The internal reference to the linked file.
	File *coal.ID `json:"-" bson:"file_id"`

	// The internal information about the linked file.
	FileType   string `json:"-" bson:"type"`
	FileLength int64  `json:"-" bson:"length"`
}

// Validate will validate the link.
func (l *Link) Validate(name string, whitelist ...string) error {
	// check file
	if l.File == nil || l.File.IsZero() {
		return fire.E("%s invalid file", name)
	}

	// check type
	err := ValidateType(l.FileType, whitelist...)
	if err != nil {
		return fire.E("%s %w", name, err)
	}

	// check length
	if l.FileLength <= 0 {
		return fire.E("%s zero length", name)
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

	// The MIME type of the file e.g. "image/png".
	Type string `json:"type"`

	// The total length of the file.
	Length int64 `json:"length"`

	// The service specific blob handle.
	Handle Handle `json:"handle"`
}

// Validate will validate the model.
func (f *File) Validate() error {
	// check state
	if !f.State.Valid() {
		return fmt.Errorf("invalid state")
	}

	return nil
}

// AddFileIndexes will add files indexes to the specified catalog.
func AddFileIndexes(catalog *coal.Catalog) {
	// index state for fast lookups
	catalog.AddIndex(&File{}, false, 0, "State", "Updated")
}

// ValidateType will validate a content/media type.
func ValidateType(typ string, whitelist ...string) error {
	// check media type
	typ, _, err := mime.ParseMediaType(typ)
	if err != nil {
		return fire.E("type invalid")
	} else if len(whitelist) > 0 && !stick.Contains(whitelist, typ) {
		return fire.E("type unallowed")
	}

	return nil
}
