package status

import (
	"io"
	"sync"
)

// Extension defines an interface for the cluster status extension
type Extension interface {
	// Collect collects extended cluster status information
	Collect() error
	// WriterTo allows to write extended cluster status into provided writer
	io.WriterTo
}

// NewExtensionFunc defines a function that returns a new instance
// of a status extension
type NewExtensionFunc func() Extension

// SetExtension sets the status collector extension
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
func (e *defaultExtension) Collect() error { return nil }

// WriteTo is no-op for the default extension
func (e *defaultExtension) WriteTo(io.Writer) (int64, error) { return 0, nil }
