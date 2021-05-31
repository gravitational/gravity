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
	"time"

	"github.com/gravitational/trace"
)

// newMultiBolt returns a bolt engine that supports simultaneous usage by
// multiple clients
//
// This is achieved by opening/closing the database file on each operation
// because in regular mode bolt keeps an exclusive lock on the file.
func newMultiBolt(cfg BoltConfig) *multiBolt {
	return &multiBolt{
		cfg: cfg,
	}
}

type multiBolt struct {
	cfg BoltConfig
}

func (b *multiBolt) createDir(key key, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.createDir(key, ttl))
	}))
}

func (b *multiBolt) upsertDir(key key, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.upsertDir(key, ttl))
	}))
}

func (b *multiBolt) deleteDir(key key) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.deleteDir(key))
	}))
}

func (b *multiBolt) createVal(key key, val interface{}, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.createVal(key, val, ttl))
	}))
}

func (b *multiBolt) createValBytes(key key, data []byte, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.createValBytes(key, data, ttl))
	}))
}

func (b *multiBolt) upsertVal(key key, val interface{}, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.upsertVal(key, val, ttl))
	}))
}

func (b *multiBolt) upsertValBytes(key key, val []byte, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.upsertValBytes(key, val, ttl))
	}))
}

func (b *multiBolt) updateVal(key key, val interface{}, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.updateVal(key, val, ttl))
	}))
}

func (b *multiBolt) updateValBytes(key key, data []byte, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.updateValBytes(key, data, ttl))
	}))
}

func (b *multiBolt) updateTTL(key key, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.updateTTL(key, ttl))
	}))
}

func (b *multiBolt) compareAndSwap(key key, val interface{}, prevVal interface{}, newVal interface{}, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.compareAndSwap(key, val, prevVal, newVal, ttl))
	}))
}

func (b *multiBolt) compareAndSwapBytes(key key, val, prevVal []byte, outVal *[]byte, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.compareAndSwapBytes(key, val, prevVal, outVal, ttl))
	}))
}

func (b *multiBolt) getVal(key key, val interface{}) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.getVal(key, val))
	}))
}

func (b *multiBolt) getValBytes(key key) (bytes []byte, err error) {
	err = b.withBolt(func(b *blt) error {
		bytes, err = b.getValBytes(key)
		return trace.Wrap(err)
	})
	return bytes, trace.Wrap(err)
}

func (b *multiBolt) deleteKey(key key) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.deleteKey(key))
	}))
}

func (b *multiBolt) compareAndDelete(key key, prevVal interface{}) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.compareAndDelete(key, prevVal))
	}))
}

func (b *multiBolt) acquireLock(token key, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.acquireLock(token, ttl))
	}))
}

func (b *multiBolt) tryAcquireLock(token key, ttl time.Duration) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.tryAcquireLock(token, ttl))
	}))
}

func (b *multiBolt) releaseLock(token key) error {
	return trace.Wrap(b.withBolt(func(b *blt) error {
		return trace.Wrap(b.releaseLock(token))
	}))
}

func (b *multiBolt) getKeys(key key) (keys []string, err error) {
	err = b.withBolt(func(b *blt) error {
		keys, err = b.getKeys(key)
		return trace.Wrap(err)
	})
	return keys, trace.Wrap(err)
}

func (b *multiBolt) key(prefix string, keys ...string) key {
	return append([]string{"root", prefix}, keys...)
}

func (b *multiBolt) Close() error {
	// no-op since all APIs implicitly close the database
	return nil
}

func (b *multiBolt) withBolt(fn func(b *blt) error) error {
	bolt, err := newBolt(b.cfg, &v1codec{})
	if err != nil {
		return trace.Wrap(err)
	}
	defer bolt.Close()
	return trace.Wrap(fn(bolt))
}
