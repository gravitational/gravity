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

package utils

import (
	"bytes"
	"fmt"
	"io"

	"crypto/sha512"
)

// SHA512Half is a first half of SHA512 hash of the byte string
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
