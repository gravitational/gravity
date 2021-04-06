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
	"io"
	"time"
)

type kvengine interface {
	io.Closer
	key(prefix string, keys ...string) key
	createVal(key key, val interface{}, ttl time.Duration) error
	createValBytes(key key, data []byte, ttl time.Duration) error
	upsertVal(key key, val interface{}, ttl time.Duration) error
	upsertValBytes(key key, val []byte, ttl time.Duration) error
	updateVal(key key, val interface{}, ttl time.Duration) error
	updateValBytes(key key, data []byte, ttl time.Duration) error
	updateTTL(key key, ttl time.Duration) error
	compareAndSwap(key key, val, prevVal, outVal interface{}, ttl time.Duration) error
	compareAndSwapBytes(key key, val, prevVal []byte, outVal *[]byte, ttl time.Duration) error
	getVal(key key, val interface{}) error
	getValBytes(key key) ([]byte, error)
	deleteKey(key key) error
	// compareAndDelete deletes specified key only if the given value matches its contents
	compareAndDelete(key key, prevVal interface{}) error
	createDir(key key, ttl time.Duration) error
	upsertDir(key key, ttl time.Duration) error
	deleteDir(key key) error
	acquireLock(token key, ttl time.Duration) error
	tryAcquireLock(token key, ttl time.Duration) error
	releaseLock(token key) error
	getKeys(key key) ([]string, error)
}

type key []string
