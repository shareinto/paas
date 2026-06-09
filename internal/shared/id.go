package shared

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
)

// ID is the common identifier type used across modules.
type ID string

func (id ID) String() string {
	return string(id)
}

func (id ID) IsZero() bool {
	return strings.TrimSpace(string(id)) == ""
}

type IDGenerator interface {
	NewID(prefix string) (ID, error)
}

type RandomIDGenerator struct{}

func (RandomIDGenerator) NewID(prefix string) (ID, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", errors.New("id prefix is required")
	}

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}

	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:])
	return ID(prefix + "_" + strings.ToLower(encoded)), nil
}
