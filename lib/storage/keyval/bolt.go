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
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/boltdb/bolt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// NewBolt returns new BoltDB-backed engine
func NewBolt(cfg BoltConfig) (storage.Backend, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var engine kvengine
	if cfg.Multi {
		engine, err = newMultiBolt(cfg)
	} else {
		engine, err = newBolt(cfg, &v1codec{})
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clock := cfg.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &backend{
		Clock:    clock,
		kvengine: engine,

		cachedCompleteOperations: make(map[string]*storage.SiteOperation),
		cachedPlanChange:         make(map[string]*storage.PlanChange),
	}, nil
}

// BoltConfig is a BoltDB configuration
type BoltConfig struct {
	// Path is a path to DB file
	Path string `json:"path"`
	// Clock is a clock interface, used in tests
	Clock clockwork.Clock `json:"-"`
	// Readonly sets bolt to read only mode
	Readonly bool `json:"readonly"`
	// Multi enables multi-client support
	Multi bool `json:"multi"`
	// When left unspecified, it will block for maximum of defaults.DBOpenTimeout.
	// When set to a negative duration, it will fail immediately if the file is already locked.
	// This option is only available on Darwin and Linux.
	// Use NoTimeout to make the operation non-blocking
	Timeout time.Duration
}

// NoTimeout defines a special duration value indicating that the blocking operation
// should not block
const NoTimeout = -1

// CheckAndSetDefaults validates this configuration and sets defaults
func (b *BoltConfig) CheckAndSetDefaults() error {
	if b.Path == "" {
		return trace.BadParameter("missing Path parameter")
	}
	path, err := filepath.Abs(b.Path)
	if err != nil {
		return trace.Wrap(err, "expected a valid path")
	}
	dir := filepath.Dir(path)
	s, err := os.Stat(dir)
	if err != nil {
		return trace.Wrap(err)
	}
	if !s.IsDir() {
		return trace.BadParameter("path '%v' should be a valid directory", dir)
	}
	if b.Timeout == 0 {
		b.Timeout = defaults.DBOpenTimeout
	}
	return nil
}

// blt is a BoltDB-backend engine
type blt struct {
	sync.Mutex
	logrus.FieldLogger

	codec Codec
	db    *bolt.DB
	clock clockwork.Clock
	path  string
	locks map[string]time.Time
}

// newBolt returns a new instance of BoltDB backend
func newBolt(cfg BoltConfig, codec Codec) (*blt, error) {
	path, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	b := &blt{
		locks: make(map[string]time.Time),
		clock: cfg.Clock,
		codec: codec,
		path:  path,
		FieldLogger: logrus.WithFields(logrus.Fields{
			trace.Component: "boltdb",
			"path":          path,
		}),
	}
	if b.clock == nil {
		b.clock = clockwork.NewRealClock()
	}

	// When opening bolt in read-only mode, make sure bolt properly initializes
	// the database file in case no database file exists before applying
	// read-only mode
	if cfg.Readonly {
		err := b.initDatafile(path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	err = b.open(cfg.Readonly, cfg.Timeout)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return b, nil
}

func (b *blt) open(readonly bool, timeout time.Duration) error {
	b.Lock()
	defer b.Unlock()
	if b.db != nil {
		return trace.AlreadyExists("database %v is already open", b.path)
	}
	db, err := bolt.Open(b.path, defaults.PrivateFileMask, &bolt.Options{
		Timeout:  timeout,
		ReadOnly: readonly,
	})
	if err != nil {
		if err == bolt.ErrTimeout {
			return trace.ConnectionProblem(err,
				"database %v is locked, is another instance running?", b.path)
		}
		// bolt needs mmap so when running on a filesystem that doesn't support
		// it, the mmap call fails with errno == "invalid value"
		// example of an unsupported filesystem is vboxsf - VirtualBox shared
		// folder (https://www.virtualbox.org/ticket/819#comment:61)
		if err == syscall.EINVAL {
			return utils.NewUnsupportedFilesystemError(err, filepath.Dir(b.path))
		}
		return trace.Wrap(err)
	}
	b.db = db
	return nil
}

func (b *blt) initDatafile(path string) error {
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return trace.ConvertSystemError(err)
	}
	if os.IsNotExist(err) {
		db, err := bolt.Open(path, defaults.PrivateFileMask, &bolt.Options{
			Timeout: defaults.DBOpenTimeout,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer db.Close()
		b.Debug("Initialized datafile.")
	}
	return nil
}

func (b *blt) key(prefix string, keys ...string) key {
	return append([]string{"root", prefix}, keys...)
}

func (b *blt) split(key key) ([]string, string) {
	return key[:len(key)-1], key[len(key)-1]
}

func upsertBucket(tx *bolt.Tx, buckets []string) (*bolt.Bucket, error) {
	bkt, err := tx.CreateBucketIfNotExists([]byte(buckets[0]))
	if err != nil {
		return nil, trace.Wrap(boltErr(err))
	}
	for _, key := range buckets[1:] {
		bkt, err = bkt.CreateBucketIfNotExists([]byte(key))
		if err != nil {
			return nil, trace.Wrap(boltErr(err))
		}
	}
	return bkt, nil
}

func createBucket(tx *bolt.Tx, buckets []string) (*bolt.Bucket, error) {
	bkt, err := tx.CreateBucketIfNotExists([]byte(buckets[0]))
	if err != nil {
		return nil, trace.Wrap(boltErr(err))
	}
	rest := buckets[1:]
	for i, key := range rest {
		if i == len(rest)-1 {
			bkt, err = bkt.CreateBucket([]byte(key))
			if err != nil {
				return nil, trace.Wrap(boltErr(err))
			}
		} else {
			bkt, err = bkt.CreateBucketIfNotExists([]byte(key))
			if err != nil {
				return nil, trace.Wrap(boltErr(err))
			}
		}
	}
	return bkt, nil
}

func getBucket(tx *bolt.Tx, buckets []string) (*bolt.Bucket, error) {
	bkt := tx.Bucket([]byte(buckets[0]))
	if bkt == nil {
		return nil, trace.NotFound("bucket %v not found", buckets[0])
	}
	for _, key := range buckets[1:] {
		bkt = bkt.Bucket([]byte(key))
		if bkt == nil {
			return nil, trace.NotFound("bucket %v not found", key)
		}
	}
	return bkt, nil
}

func (b *blt) createDir(key key, ttl time.Duration) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		_, err := createBucket(tx, key)
		return trace.Wrap(boltErr(err))
	})
}

func (b *blt) upsertDir(key key, ttl time.Duration) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		_, err := upsertBucket(tx, key)
		return trace.Wrap(boltErr(err))
	})
}

func (b *blt) createValBytes(k key, data []byte, ttl time.Duration) error {
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := upsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		val := bkt.Get([]byte(key))
		if val != nil {
			return trace.AlreadyExists("%v already exists", key)
		}
		return bkt.Put([]byte(key), data)
	})
}

func (b *blt) createVal(k key, val interface{}, ttl time.Duration) error {
	encoded, err := b.codec.EncodeToBytes(val)
	if err != nil {
		return trace.Wrap(err)
	}
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := upsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		val := bkt.Get([]byte(key))
		if val != nil {
			return trace.AlreadyExists("'%v' already exists", key)
		}
		return bkt.Put([]byte(key), encoded)
	})
}

func (b *blt) upsertValBytes(k key, encoded []byte, ttl time.Duration) error {
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := upsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		return bkt.Put([]byte(key), encoded)
	})
}

func (b *blt) upsertVal(k key, val interface{}, ttl time.Duration) error {
	encoded, err := b.codec.EncodeToBytes(val)
	if err != nil {
		return trace.Wrap(err)
	}
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := upsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		return bkt.Put([]byte(key), encoded)
	})
}

func (b *blt) updateValBytes(k key, data []byte, ttl time.Duration) error {
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := upsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		val := bkt.Get([]byte(key))
		if val == nil {
			return trace.NotFound("%q not found", key)
		}
		return bkt.Put([]byte(key), data)
	})
}

func (b *blt) updateVal(k key, val interface{}, ttl time.Duration) error {
	encoded, err := b.codec.EncodeToBytes(val)
	if err != nil {
		return trace.Wrap(err)
	}
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := upsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		val := bkt.Get([]byte(key))
		if val == nil {
			return trace.NotFound("%q not found", key)
		}
		return bkt.Put([]byte(key), encoded)
	})
}

func (b *blt) updateTTL(k key, ttl time.Duration) error {
	// Not supported
	return nil
}

func (b *blt) compareAndSwap(k key, val interface{}, prevVal interface{}, outVal interface{}, ttl time.Duration) error {
	encoded, err := b.codec.EncodeToBytes(val)
	if err != nil {
		return trace.Wrap(err)
	}
	var prevEncoded []byte
	if prevVal != nil {
		prevEncoded, err = b.codec.EncodeToBytes(prevVal)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	var outEncoded []byte
	err = b.compareAndSwapBytes(k, encoded, prevEncoded, &outEncoded, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	if prevVal != nil {
		err = b.codec.DecodeFromBytes(outEncoded, outVal)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (b *blt) compareAndSwapBytes(k key, val, prevVal []byte, outVal *[]byte, ttl time.Duration) error {
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := upsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		currentVal := bkt.Get([]byte(key))
		if prevVal == nil { // we don't expect the value to exist
			if currentVal != nil {
				return trace.AlreadyExists("key %q already exists", key)
			}
			return trace.Wrap(bkt.Put([]byte(key), val))
		} else { // we expect the previous value to exist
			if val == nil {
				return trace.NotFound("key %q not found", key)
			}
			if !bytes.Equal(currentVal, prevVal) {
				return trace.CompareFailed("expected %q got %q",
					string(prevVal), string(currentVal))
			}
			err = bkt.Put([]byte(key), val)
			if err != nil {
				return trace.Wrap(err)
			}
			*outVal = currentVal
			return nil
		}
	})
}

func (b *blt) getValBytes(k key) ([]byte, error) {
	buckets, key := b.split(k)
	var out []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		bytes := bkt.Get([]byte(key))
		if bytes == nil {
			_, err := getBucket(tx, append(buckets, key))
			if err == nil {
				return trace.BadParameter("key %q is a bucket", key)
			}
			return trace.NotFound("%q %q not found", buckets, key)
		}
		out = make([]byte, len(bytes))
		copy(out, bytes)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

func (b *blt) getVal(k key, outVal interface{}) error {
	buckets, key := b.split(k)
	return b.db.View(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		bytes := bkt.Get([]byte(key))
		if bytes == nil {
			_, err := getBucket(tx, append(buckets, key))
			if err == nil {
				return trace.BadParameter("key '%v 'is a bucket", key)
			}
			return trace.NotFound("%v %v not found", buckets, key)
		}
		return b.codec.DecodeFromBytes(bytes, outVal)
	})
}

func (b *blt) compareAndDelete(k key, prevVal interface{}) error {
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		bytes := bkt.Get([]byte(key))
		if bytes == nil {
			return trace.NotFound("%v is not found", key)
		}
		var outVal interface{}
		err = b.codec.DecodeFromBytes(bytes, &outVal)
		if err != nil {
			return trace.Wrap(err)
		}
		if outVal != prevVal {
			return trace.BadParameter("%v: expected %v, but got %v", key, prevVal, outVal)
		}
		return bkt.Delete([]byte(key))
	})
}

func (b *blt) deleteKey(k key) error {
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		if bkt.Get([]byte(key)) == nil {
			return trace.NotFound("%v is not found", key)
		}
		return bkt.Delete([]byte(key))
	})
}

func (b *blt) deleteDir(k key) error {
	buckets, key := b.split(k)
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		err = bkt.DeleteBucket([]byte(key))
		if err != nil {
			return trace.NotFound("%v is not found", key)
		}
		return nil
	})
}

func (b *blt) acquireLock(token key, ttl time.Duration) error {
	for {
		err := b.tryAcquireLock(token, ttl)
		if err != nil {
			if !trace.IsCompareFailed(err) && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
			time.Sleep(delayBetweenLockAttempts)
		} else {
			return nil
		}
	}
}

func (b *blt) tryAcquireLock(key key, ttl time.Duration) error {
	return b.createVal(key, "locked", ttl)
}

func (b *blt) releaseLock(key key) error {
	return b.deleteKey(key)
}

func (b *blt) getKeys(key key) ([]string, error) {
	out := []string{}
	buckets := key
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		c := bkt.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			out = append(out, string(k))
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Strings(out)
	return out, nil
}

// Close closes the backend resources
func (b *blt) Close() error {
	b.Lock()
	defer b.Unlock()
	if b.db == nil {
		return trace.AlreadyExists("database %v is already closed", b.path)
	}
	err := b.db.Close()
	if err != nil {
		return trace.Wrap(err)
	}
	b.db = nil
	return nil
}

func boltErr(err error) error {
	if err == bolt.ErrBucketNotFound {
		return trace.NotFound(err.Error())
	}
	if err == bolt.ErrBucketExists {
		return trace.AlreadyExists(err.Error())
	}
	return err
}
