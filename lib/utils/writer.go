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
	"io"
	"net"

	"github.com/gravitational/tail"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func NewTailReader(path string) (io.ReadCloser, error) {
	t, err := tail.TailFile(path, tail.Config{Follow: true})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	in, out := net.Pipe()
	go func() {
		for line := range t.Lines {
			_, err := io.WriteString(in, line.Text+"\n")
			if err != nil {
				return
			}
		}
	}()
	return &TailReader{reader: t, in: in, out: out}, nil
}

type TailReader struct {
	reader *tail.Tail
	in     net.Conn
	out    net.Conn
}

func (t *TailReader) Read(p []byte) (int, error) {
	return t.out.Read(p)
}

func (t *TailReader) Close() error {
	log.Infof("TailReader closing")
	defer t.in.Close()
	defer t.out.Close()
	return t.reader.Stop()
}

// NewMultiWriteCloser returns new WriteCloser, all writes go to all
// writers one by one and close closes all closers one by one
func NewMultiWriteCloser(writeClosers ...io.WriteCloser) io.WriteCloser {
	writers := make([]io.Writer, len(writeClosers))
	closers := make([]io.Closer, len(writeClosers))
	for i := range writeClosers {
		closers[i] = writeClosers[i]
		writers[i] = writeClosers[i]
	}
	return &MultiWriteCloser{
		Writer:      io.MultiWriter(writers...),
		MultiCloser: closers,
	}
}

// MultiWriteCloser returns multi writer and multi closer
type MultiWriteCloser struct {
	io.Writer
	MultiCloser
}

// Close closes all closers one by one
func (m *MultiWriteCloser) Close() error {
	return m.MultiCloser.Close()
}

// MultiCloser is a list of closers with combined Close method
type MultiCloser []io.Closer

// Close closes all closers one by one
func (m MultiCloser) Close() error {
	var errors []error
	for _, c := range m {
		err := c.Close()
		if err != nil {
			errors = append(errors, trace.Wrap(err))
		}
	}
	return trace.NewAggregate(errors...)
}

// NopReader returns a new no-op io.Reader
func NopReader() *nopReader {
	return &nopReader{}
}

// nopReader is a io.Reader with no-op Read method
type nopReader struct{}

// Read is no-op, always returns 0
func (r *nopReader) Read(_ []byte) (n int, err error) {
	return 0, io.EOF
}

// NopWriteCloser returns a WriteCloser with a no-op Close method wrapping
// the provided Writer w.
func NopWriteCloser(w io.Writer) io.WriteCloser {
	return nopWriteCloser{w}
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
