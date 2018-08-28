package blob

import (
	"io"
	"time"
)

// Envelope specifies the metadata about BLOB - it's SHA512 hash and size
type Envelope struct {
	// SizeBytes is the BLOB size in bytes
	SizeBytes int64 `json:"size_bytes"`
	// SHA512 is the half SHA512 hash of the BLOB
	SHA512 string `json:"sha512"`
	// Modified specifies the time this file was last modified
	Modified time.Time `json:"modified"`
}

// ReadSeekCloser implements Reader, Seeker and Closer
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// Objects is a large BLOB Object storage
// implemented by some backends
type Objects interface {
	io.Closer
	// WriteBLOB writes BLOB to storage, on success
	// returns the envelope with hash of the blob
	WriteBLOB(data io.Reader) (*Envelope, error)
	// OpenBLOB opens the BLOB by hash and returns reader object
	OpenBLOB(hash string) (ReadSeekCloser, error)
	// DeleteBLOB deletes the blob by hash
	DeleteBLOB(hash string) error
	// GetBLOBs returns blobs list present in the store
	GetBLOBs() ([]string, error)
	// GetBLOBEnvelope returns BLOB envelope
	GetBLOBEnvelope(hash string) (*Envelope, error)
}
