package iface

import (
	"math/rand"
)

// ID identifies a face.
// Zero ID is invalid.
type ID uint16

// ID limits.
const (
	MinID = 0x1000
	MaxID = 0xEFFF
)

// AllocID allocates a random ID.
// Warning: endless loop if all possible IDs are used up.
func AllocID() (id ID) {
	for !id.Valid() || gFaces[id] != nil {
		id = ID(rand.Uint32())
	}
	return id
}

// Valid determines whether id is valid.
func (id ID) Valid() bool {
	return id >= MinID && id <= MaxID
}
