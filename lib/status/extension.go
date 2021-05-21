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

package status

import (
	"context"
	"io"
	"sync"
)

// Extension defines an interface for the cluster status extension
type Extension interface {
	// Collect collects extended cluster status information
	Collect(context.Context) error
	// WriterTo allows to write extended cluster status into provided writer
	io.WriterTo
}

// NewExtensionFunc defines a function that returns a new instance
// of a status extension
type NewExtensionFunc func() Extension

// SetExtensionFunc sets the status collector extension
func SetExtensionFunc(f NewExtensionFunc) {
	mutex.Lock()
	defer mutex.Unlock()
	newExtensionFunc = f
}

// newExtension returns a new instance of status collector extension
func newExtension() Extension {
	mutex.Lock()
	defer mutex.Unlock()
	return newExtensionFunc()
}

var (
	mutex                             = &sync.Mutex{}
	newExtensionFunc NewExtensionFunc = func() Extension {
		return &defaultExtension{}
	}
)

type defaultExtension struct{}

// Collect is no-op for the default extension
func (*defaultExtension) Collect(context.Context) error { return nil }

// WriteTo is no-op for the default extension
func (*defaultExtension) WriteTo(io.Writer) (int64, error) { return 0, nil }
