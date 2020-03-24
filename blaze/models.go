package blaze

import (
	"fmt"
	"time"

	"github.com/256dpi/fire/coal"
)

// Blob may be used instead of a bytes slice for inline storage if the type
// needs be stored as well.
type Blob struct {
	Type  string `json:"type"`
	Bytes []byte `json:"bytes"`
}

// Link is used to link a file to a model.
type Link struct {
	// The type of the linked file.
	Type string `json:"type"`

	// The length of the linked file.
	Length int64 `json:"length"`

	// The key for claiming the linked file.
	ClaimKey string `json:"claim-key" bson:"-"`

	// The key for viewing the linked file.
	ViewKey string `json:"view-key" bson:"-"`

	// The reference to the linked file.
	File *coal.ID `json:"-" bson:"file_id"`
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
