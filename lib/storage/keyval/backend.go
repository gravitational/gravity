/*
Copyright 2018-2021 Gravitational, Inc.

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

package keyval

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// backend implements storage interface, it also acts as a codec
type backend struct {
	clockwork.Clock
	kvengine

	cachedCompleteOperationsMutex sync.RWMutex
	cachedCompleteOperations      map[string]*storage.SiteOperation

	cachedPlanChangeMutex sync.RWMutex
	cachedPlanChange      map[string]*storage.PlanChange
}

func (b *backend) ttl(t time.Time) time.Duration {
	return ttl(b, t)
}

func ttl(clock clockwork.Clock, t time.Time) time.Duration {
	if t.IsZero() {
		return forever
	}
	diff := t.UTC().Sub(clock.Now().UTC())
	if diff < 0 {
		return forever
	}
	return diff
}

func (b *backend) Close() error {
	return b.kvengine.Close()
}

// Codec is responsible for encoding/decoding objects
type Codec interface {
	EncodeToString(val interface{}) (string, error)
	EncodeBytesToString(val []byte) (string, error)
	EncodeToBytes(val interface{}) ([]byte, error)
	DecodeFromString(val string, in interface{}) error
	DecodeBytesFromString(val string) ([]byte, error)
	DecodeFromBytes(val []byte, in interface{}) error
}

// v1codec is codec designed for etcd 2.x series that don't
// reliably support binary data, so it adds additional base64 encoding
// to JSON-serialized values. We can drop this support once we move to 3.x
// series
type v1codec struct {
}

func (*v1codec) EncodeBytesToString(data []byte) (string, error) {
	data, err := compress(data)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func (*v1codec) EncodeToString(val interface{}) (string, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return "", trace.Wrap(err, "failed to encode object")
	}

	data, err = compress(data)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func (*v1codec) EncodeToBytes(val interface{}) ([]byte, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return nil, trace.Wrap(err, "failed to encode object")
	}

	data, err = compress(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

func (*v1codec) DecodeBytesFromString(val string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return nil, trace.Wrap(err, "failed to decode object")
	}

	data, err = decompress(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

func (*v1codec) DecodeFromString(val string, in interface{}) error {
	data, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return trace.Wrap(err, "failed to decode object")
	}

	data, err = decompress(data)
	if err != nil {
		return trace.Wrap(err)
	}

	err = json.Unmarshal([]byte(data), &in)
	if err != nil {
		log.Errorf("failed to decode: %s", data)
		return trace.Wrap(err)
	}
	return nil
}

func (*v1codec) DecodeFromBytes(data []byte, in interface{}) error {
	data, err := decompress(data)
	if err != nil {
		return trace.Wrap(err)
	}

	err = json.Unmarshal(data, &in)
	if err != nil {
		log.Errorf("failed to decode: %s", data)
		return trace.Wrap(err)
	}
	return nil
}

// compressThresholdB is the threshold used to determine when we compress objects to the backend
// The maximum data we can post to etcdv2 is 10Mb, so we need to compress any large objects
const compressThresholdB = 1024 * 1024 * 6

// compress will gzip compress the input data if the slice is above a threshold. Data below the threshold will
// be returned without modification.
func compress(in []byte) ([]byte, error) {
	if len(in) < 2 {
		return in, nil
	}

	// if the data is already compressed (with gzip) don't compress a second time
	// insert our own magic number to indicate to return the value as is
	if (in[0] == 0x1f && in[1] == 0x8b) || (in[0] == 0x1f && in[1] == 0x8c) {
		return append([]byte{0x1f, 0x8c}, in...), nil
	}

	if len(in) < compressThresholdB {
		return in, nil
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	_, err := gz.Write(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := gz.Close(); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

// decompress will detect and decompress gzip encoded bytes. If the data is not compressed, it will be returned as is.
func decompress(in []byte) ([]byte, error) {
	if len(in) < 2 {
		return in, nil
	}

	// check magic for if the data was compressed outside of the storage lib
	// if it was already compressed, we remove our magic number and return
	if in[0] == 0x1f && in[1] == 0x8c {
		return in[2:], nil
	}

	// gzip magic number is 0x1f8b, so if this isn't gzip data, just return it as is
	if in[0] != 0x1f || in[1] != 0x8b {
		return in, nil
	}

	gz, err := gzip.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// No security impact, the only data decompressed is data we originally compressed
	/* #nosec */
	buf, err := ioutil.ReadAll(gz)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := gz.Close(); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf, nil
}
