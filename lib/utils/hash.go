package utils

import (
	"bytes"
	"fmt"
	"io"

	"crypto/sha512"
)

// SHA512 half is a first half of SHA512 hash of the byte string
func SHA512Half(v []byte) (string, error) {
	h := sha512.New()
	_, err := io.Copy(h, bytes.NewBuffer(v))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:sha512.Size/2]), nil
}

// MustSHA512Half panics if it fails to compute SHA512 hash,
// use only in tests
func MustSHA512Half(v []byte) string {
	h, err := SHA512Half(v)
	if err != nil {
		panic(err)
	}
	return h
}
