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
	compareAndSwap(key key, val interface{}, prevVal interface{}, newVal interface{}, ttl time.Duration) error
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

func (k key) split() ([]string, string) {
	if len(k) == 0 {
		return k, ""
	}
	return k[:len(k)-1], k[len(k)-1]
}
