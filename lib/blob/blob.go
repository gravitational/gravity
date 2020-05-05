/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package blob

import (
	"fmt"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/constants"
)

// String returns text representation of this blob envelope
func (r Envelope) String() string {
	return fmt.Sprintf("blob(size=%v, hash=%v, modified=%v)",
		r.SizeBytes, r.SHA512, r.Modified.Format(constants.ShortDateFormat))
}

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
