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
	"io"
)

// NewSyncBuffer returns a new sync buffer backed by a bytes.Buffer
func NewSyncBuffer() *SyncBytesBuffer {
	var buf bytes.Buffer
	return &SyncBytesBuffer{
		b:   NewSyncBufferWithWriter(&buf),
		buf: &buf,
	}
}

// NewSyncBufferWithWriter creates a new sync buffer for the specified writer
func NewSyncBufferWithWriter(w io.Writer) *SyncBuffer {
	reader, writer := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(w, reader)
		errCh <- err
		close(errCh)
	}()
	return &SyncBuffer{
		reader: reader,
		writer: writer,
		w:      w,
		errCh:  errCh,
	}
}

// Write writes the specified data into the underlying writer.
// Implements io.Writer
func (b *SyncBuffer) Write(data []byte) (n int, err error) {
	return b.writer.Write(data)
}

// Close closes reads and writes on the buffer.
// Implements io.Closer
func (b *SyncBuffer) Close() error {
	b.reader.Close()
	b.writer.Close()
	return <-b.errCh
}

// SyncBuffer is in memory bytes buffer that is
// safe for concurrent writes
type SyncBuffer struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	w      io.Writer
	errCh  <-chan error
}

// SyncBytesBuffer is an in-memory buffer backed by bytes.Buffer
// and implemented as a SyncBuffer
type SyncBytesBuffer struct {
	b   *SyncBuffer
	buf *bytes.Buffer
}

// Write writes the specified data into the underlying writer.
// Implements io.Writer
func (b *SyncBytesBuffer) Write(data []byte) (n int, err error) {
	return b.b.Write(data)
}

// Close closes reads and writes on the buffer.
// Implements io.Closer
func (b *SyncBytesBuffer) Close() error {
	return b.b.Close()
}

// String returns contents of the buffer
// after this call, all writes will fail
func (b *SyncBytesBuffer) String() string {
	b.b.Close()
	return b.buf.String()
}

// Bytes returns contents of the buffer
// after this call, all writes will fail
func (b *SyncBytesBuffer) Bytes() []byte {
	b.b.Close()
	return b.buf.Bytes()
}
