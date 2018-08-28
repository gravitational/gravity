package utils

import (
	"bytes"
	"io"
)

// NewSyncBuffer returns new in memory buffer
func NewSyncBuffer() *SyncBuffer {
	reader, writer := io.Pipe()
	buf := &bytes.Buffer{}
	go func() {
		io.Copy(buf, reader)
	}()
	return &SyncBuffer{
		reader: reader,
		writer: writer,
		buf:    buf,
	}
}

// SyncBuffer is in memory bytes buffer that is
// safe for concurrent writes
type SyncBuffer struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	buf    *bytes.Buffer
}

func (b *SyncBuffer) Write(data []byte) (n int, err error) {
	return b.writer.Write(data)
}

// String returns contents of the buffer
// after this call, all writes will fail
func (b *SyncBuffer) String() string {
	b.Close()
	return b.buf.String()
}

// Bytes returns contents of the buffer
// after this call, all writes will fail
func (b *SyncBuffer) Bytes() []byte {
	b.Close()
	return b.buf.Bytes()
}

// Close closes reads and writes on the buffer
func (b *SyncBuffer) Close() error {
	err := b.reader.Close()
	err2 := b.writer.Close()
	if err != nil {
		return err
	}
	return err2
}
