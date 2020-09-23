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

package keyval

import (
	"encoding/base64"
	"encoding/json"
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

	cachedCompleteOperations      map[string]*storage.SiteOperation
	cachedCompleteOperationsMutex sync.RWMutex
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
	return base64.StdEncoding.EncodeToString(data), nil
}

func (*v1codec) EncodeToString(val interface{}) (string, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return "", trace.Wrap(err, "failed to encode object")
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (*v1codec) EncodeToBytes(val interface{}) ([]byte, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return nil, trace.Wrap(err, "failed to encode object")
	}
	return data, nil
}

func (*v1codec) DecodeBytesFromString(val string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return nil, trace.Wrap(err, "failed to decode object")
	}
	return data, nil
}

func (*v1codec) DecodeFromString(val string, in interface{}) error {
	data, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return trace.Wrap(err, "failed to decode object")
	}
	err = json.Unmarshal([]byte(data), &in)
	if err != nil {
		log.Errorf("failed to decode: %s", data)
		return trace.Wrap(err)
	}
	return nil
}

func (*v1codec) DecodeFromBytes(data []byte, in interface{}) error {
	err := json.Unmarshal(data, &in)
	if err != nil {
		log.Errorf("failed to decode: %s", data)
		return trace.Wrap(err)
	}
	return nil
}
