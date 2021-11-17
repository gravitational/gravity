/*
Copyright 2021 Gravitational, Inc.

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
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncBuffer(t *testing.T) {
	// setup
	b := NewSyncBuffer()
	defer func() {
		assert.Nil(t, b.Close())
	}()

	// execute
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := io.WriteString(b, "Brown fox jumps over the lazy dog.\n")
			assert.Nil(t, err)
			wg.Done()
		}()
	}
	wg.Wait()

	// verify
	expected := `Brown fox jumps over the lazy dog.
Brown fox jumps over the lazy dog.
Brown fox jumps over the lazy dog.
Brown fox jumps over the lazy dog.
Brown fox jumps over the lazy dog.
Brown fox jumps over the lazy dog.
Brown fox jumps over the lazy dog.
Brown fox jumps over the lazy dog.
Brown fox jumps over the lazy dog.
Brown fox jumps over the lazy dog.
`
	assert.Equal(t, b.String(), expected)
}
