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

// Package keyval implements Etcd and BoltDB powered storage
package keyval

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/gravitational/coordinate/leader"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// NewETCD returns new ETCD-backed engine
func NewETCD(cfg ETCDConfig) (*electingBackend, error) {
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	engine, err := newEngine(cfg, &v1codec{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clock := cfg.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	leader, err := leader.NewClient(leader.Config{Client: engine.client, Clock: clock})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	backend := &backend{
		Clock:    clock,
		kvengine: engine,
	}
	return &electingBackend{
		Backend: backend,
		Leader:  leader,
		backend: backend,
		engine:  engine,
	}, nil
}

// ETCDConfig represents JSON config for ETCD backend
type ETCDConfig struct {
	Clock         clockwork.Clock `json:"-"`
	Nodes         []string        `json:"nodes" yaml:"nodes"`
	Key           string          `json:"key" yaml:"key"`
	TLSKeyFile    string          `json:"tls_key_file" yaml:"tls_key_file"`
	TLSCertFile   string          `json:"tls_cert_file" yaml:"tls_cert_file"`
	TLSCAFile     string          `json:"tls_ca_file" yaml:"tls_ca_file"`
	RetryInterval time.Duration   `json:"retry_interval" yaml:"retry_interval"`
}

// LocalEtcdConfig returns config for local etcd
func LocalEtcdConfig(retryTimeout time.Duration) (*ETCDConfig, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if retryTimeout == 0 {
		retryTimeout = defaults.EtcdRetryInterval
	}

	return &ETCDConfig{
		Nodes:         []string{defaults.EtcdLocalAddr},
		Key:           defaults.EtcdKey,
		TLSKeyFile:    state.Secret(stateDir, defaults.EtcdKeyFilename),
		TLSCertFile:   state.Secret(stateDir, defaults.EtcdCertFilename),
		TLSCAFile:     state.Secret(stateDir, defaults.RootCertFilename),
		RetryInterval: retryTimeout,
	}, nil
}

// EtcdBackend enables access to etcd-specific features
type EtcdBackend interface {
	storage.Backend
	// CopyWithOptions creates a copy of this backend
	// with the specified options applied
	CopyWithOptions(opts ...EtcdOption) storage.Backend
}

// EtcdOption is a functional option to configure an etcd backend
type EtcdOption func(*etcdOptions)

// WithReadQuorum specifies that reads should go through the quorum,
// e.g. return the latest committed value applied in a quorum of members
func WithReadQuorum(quorum bool) EtcdOption {
	return func(config *etcdOptions) {
		logrus.WithField("quorum", quorum).Info("Specify quorum reads.")
		config.GetOptions.Quorum = quorum
	}
}

type etcdOptions struct {
	GetOptions client.GetOptions
}

// Check checks if all the parameters are valid and sets defaults
func (cfg *ETCDConfig) Check() error {
	if len(cfg.Key) == 0 {
		return trace.BadParameter(`Key: supply a valid root key for data`)
	}
	if len(cfg.Nodes) == 0 {
		return trace.BadParameter(`Nodes: please supply a valid dictionary, e.g. {"nodes": ["http://localhost:2379]}`)
	}
	if cfg.TLSKeyFile == "" {
		return trace.BadParameter(`TLSKeyFile: please supply a path to TLS private key file`)
	}
	if cfg.TLSCertFile == "" {
		return trace.BadParameter(`TLSCertFile: please supply a path to TLS certificate file`)
	}
	return nil
}

// newEngine returns new etcd client engine with some useful wrappers
func newEngine(cfg ETCDConfig, codec Codec) (*engine, error) {
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	e := &engine{
		config:  cfg,
		nodes:   cfg.Nodes,
		etcdKey: strings.Split(cfg.Key, "/"),
		codec:   codec,
	}
	if err := e.reconnect(); err != nil {
		return nil, trace.Wrap(err)
	}
	return e, nil
}

type engine struct {
	client.KeysAPI
	nodes   []string
	codec   Codec
	config  ETCDConfig
	etcdKey []string
	client  client.Client
	options etcdOptions
}

func (e *engine) copyWithOptions(opts ...EtcdOption) *engine {
	options := e.options
	for _, opt := range opts {
		opt(&options)
	}
	// Create a shallow copy of the engine
	engine := *e
	engine.options = options
	return &engine
}

func (e *engine) key(prefix string, keys ...string) key {
	key := make([]string, 0, len(e.etcdKey)+len(keys)+1)
	key = append(key, e.etcdKey...)
	key = append(key, prefix)
	key = append(key, keys...)
	for i := range key {
		key[i] = strings.Replace(key[i], "/", "%2F", -1)
	}
	return key
}

// ekey returns etcd formatted key
func ekey(key key) string {
	out := strings.Join(key, "/")
	return out
}

func (e *engine) Close() error {
	return nil
}

func (e *engine) reconnect() error {
	info := transport.TLSInfo{
		CAFile:   e.config.TLSCAFile,
		CertFile: e.config.TLSCertFile,
		KeyFile:  e.config.TLSKeyFile,
	}
	cfg, err := info.ClientConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: defaults.DialTimeout,
			// value taken from http.DefaultTransport
			KeepAlive: defaults.KeepAliveTimeout,
		}).Dial,
		// value taken from http.DefaultTransport
		TLSHandshakeTimeout: defaults.DialTimeout,
		TLSClientConfig:     cfg,
		MaxIdleConnsPerHost: defaults.MaxIdleConnsPerHost,
	}
	clt, err := client.New(client.Config{
		Endpoints:               e.nodes,
		Transport:               transport,
		HeaderTimeoutPerRequest: defaults.ReadHeadersTimeout,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	e.client = clt
	e.KeysAPI = retryApi{
		api:      client.NewKeysAPI(e.client),
		interval: e.config.RetryInterval,
	}

	return nil
}

func (e *engine) createValBytes(key key, data []byte, ttl time.Duration) error {
	encoded, err := e.codec.EncodeBytesToString(data)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = e.Set(
		context.TODO(), ekey(key), encoded,
		&client.SetOptions{PrevExist: client.PrevNoExist, TTL: ttl})
	return trace.Wrap(convertErr(err))
}

func (e *engine) createVal(key key, val interface{}, ttl time.Duration) error {
	encoded, err := e.codec.EncodeToString(val)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = e.Set(
		context.TODO(), ekey(key), encoded,
		&client.SetOptions{PrevExist: client.PrevNoExist, TTL: ttl})
	return trace.Wrap(convertErr(err))
}

func (e *engine) createDir(key key, ttl time.Duration) error {
	_, err := e.Set(
		context.TODO(), ekey(key), "",
		&client.SetOptions{PrevExist: client.PrevNoExist, TTL: ttl, Dir: true})
	return trace.Wrap(convertErr(err))
}

func (e *engine) upsertDir(key key, ttl time.Duration) error {
	err := convertErr(e.createDir(key, ttl))
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}
	_, err = e.Set(
		context.TODO(), ekey(key), "",
		&client.SetOptions{TTL: ttl, Dir: true, PrevExist: client.PrevExist})
	return trace.Wrap(convertErr(err))
}

func (e *engine) upsertValBytes(key key, data []byte, ttl time.Duration) error {
	encoded, err := e.codec.EncodeBytesToString(data)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = e.Set(
		context.TODO(), ekey(key), encoded, &client.SetOptions{TTL: ttl})
	return convertErr(err)
}

func (e *engine) upsertVal(key key, val interface{}, ttl time.Duration) error {
	encoded, err := e.codec.EncodeToString(val)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = e.Set(
		context.TODO(), ekey(key), encoded, &client.SetOptions{TTL: ttl})
	return convertErr(err)
}

func (e *engine) updateValBytes(key key, data []byte, ttl time.Duration) error {
	encoded, err := e.codec.EncodeBytesToString(data)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = e.Set(
		context.TODO(), ekey(key), encoded, &client.SetOptions{TTL: ttl, PrevExist: client.PrevExist})
	return convertErr(err)
}

func (e *engine) updateVal(key key, val interface{}, ttl time.Duration) error {
	encoded, err := e.codec.EncodeToString(val)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = e.Set(
		context.TODO(), ekey(key), encoded, &client.SetOptions{TTL: ttl, PrevExist: client.PrevExist})
	return convertErr(err)
}

func (e *engine) updateTTL(key key, ttl time.Duration) error {
	_, err := e.Set(
		context.TODO(), ekey(key), "", &client.SetOptions{TTL: ttl, Refresh: true, PrevExist: client.PrevExist})
	return convertErr(err)
}

func (e *engine) compareAndSwap(key key, val interface{}, prevVal interface{}, outVal interface{}, ttl time.Duration) error {
	encoded, err := e.codec.EncodeToString(val)
	if err != nil {
		return trace.Wrap(err)
	}
	var re *client.Response
	var encodedPrev string
	if prevVal != nil {
		encodedPrev, err = e.codec.EncodeToString(prevVal)
		if err != nil {
			return trace.Wrap(err)
		}
		re, err = e.Set(
			context.TODO(), ekey(key), encoded,
			&client.SetOptions{TTL: ttl, PrevValue: encodedPrev, PrevExist: client.PrevExist})
	} else {
		re, err = e.Set(
			context.TODO(),
			ekey(key), encoded,
			&client.SetOptions{TTL: ttl, PrevExist: client.PrevNoExist})
	}
	err = convertErr(err)
	if err != nil {
		return trace.Wrap(err)
	}
	if re != nil && re.PrevNode != nil {
		err = e.codec.DecodeFromString(re.PrevNode.Value, outVal)
		return trace.Wrap(err)
	}
	return nil
}

func (e *engine) compareAndSwapBytes(key key, val, prevVal []byte, outVal *[]byte, ttl time.Duration) error {
	encoded, err := e.codec.EncodeBytesToString(val)
	if err != nil {
		return trace.Wrap(err)
	}
	var re *client.Response
	var encodedPrev string
	if prevVal != nil {
		encodedPrev, err = e.codec.EncodeBytesToString(prevVal)
		if err != nil {
			return trace.Wrap(err)
		}
		re, err = e.Set(
			context.TODO(), ekey(key), encoded,
			&client.SetOptions{TTL: ttl, PrevValue: encodedPrev, PrevExist: client.PrevExist})
	} else {
		re, err = e.Set(
			context.TODO(),
			ekey(key), encoded,
			&client.SetOptions{TTL: ttl, PrevExist: client.PrevNoExist})
	}
	err = convertErr(err)
	if err != nil {
		return trace.Wrap(err)
	}
	if re != nil && re.PrevNode != nil {
		*outVal, err = e.codec.DecodeBytesFromString(re.PrevNode.Value)
		return trace.Wrap(err)
	}
	return nil
}

func (e *engine) getValBytes(key key) ([]byte, error) {
	re, err := e.Get(context.TODO(), ekey(key), &e.options.GetOptions)
	if err != nil {
		return nil, convertErr(err)
	}
	if re.Node.Dir {
		return nil, trace.BadParameter("%q is not a bucket", key)
	}
	return e.codec.DecodeBytesFromString(re.Node.Value)
}

func (e *engine) getVal(key key, val interface{}) error {
	re, err := e.Get(context.TODO(), ekey(key), &e.options.GetOptions)
	if err != nil {
		return convertErr(err)
	}
	if re.Node.Dir {
		return trace.BadParameter("%q is not a bucket", key)
	}
	err = e.codec.DecodeFromString(re.Node.Value, val)
	return trace.Wrap(err)
}

func (e *engine) compareAndDelete(key key, prevVal interface{}) error {
	encoded, err := e.codec.EncodeToString(prevVal)
	if err != nil {
		return trace.Wrap(err)
	}

	opts := client.DeleteOptions{PrevValue: encoded}
	_, err = e.Delete(context.TODO(), ekey(key), &opts)
	return convertErr(err)
}

func (e *engine) deleteKey(key key) error {
	_, err := e.Delete(context.TODO(), ekey(key), nil)
	return convertErr(err)
}

func (e *engine) deleteDir(key key) error {
	_, err := e.Delete(context.TODO(), ekey(key),
		&client.DeleteOptions{Dir: true, Recursive: true})
	return convertErr(err)
}

const delayBetweenLockAttempts = 500 * time.Millisecond

func (e *engine) acquireLock(token key, ttl time.Duration) error {
	for {
		err := e.tryAcquireLock(token, ttl)
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

func (e *engine) tryAcquireLock(key key, ttl time.Duration) error {
	_, err := e.Set(
		context.TODO(),
		ekey(key), "locked",
		&client.SetOptions{TTL: ttl, PrevExist: client.PrevNoExist})
	return convertErr(err)
}

func (e *engine) releaseLock(key key) error {
	_, err := e.Delete(context.TODO(), ekey(key), nil)
	return convertErr(err)
}

func (e *engine) getKeys(key key) ([]string, error) {
	var vals []string
	re, err := e.Get(context.TODO(), ekey(key), &e.options.GetOptions)
	err = convertErr(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return vals, nil
		}
		return nil, trace.Wrap(err)
	}
	if !isDir(re.Node) {
		return nil, trace.BadParameter("'%v': expected directory", key)
	}
	for _, n := range re.Node.Nodes {
		vals = append(vals, suffix(n.Key))
	}
	sort.Sort(sort.StringSlice(vals))
	return vals, nil
}

func convertErr(e error) error {
	if e == nil {
		return nil
	}
	switch err := e.(type) {
	case *client.ClusterError:
		return trace.ConnectionProblem(err, "failed to connect to the etcd cluster")
	case client.Error:
		switch err.Code {
		case client.ErrorCodeKeyNotFound:
			return trace.NotFound(err.Error())
		case client.ErrorCodeNotFile:
			return trace.BadParameter(err.Error())
		case client.ErrorCodeNodeExist:
			return trace.AlreadyExists(err.Error())
		case client.ErrorCodeTestFailed:
			return trace.CompareFailed(err.Error())
		}
	}
	return e
}

func isDir(n *client.Node) bool {
	return n != nil && n.Dir == true
}

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[len(vals)-1]
}

// Get retrieves a set of Nodes from etcd
func (r retryApi) Get(ctx context.Context, key string, opts *client.GetOptions) (*client.Response, error) {
	resp, err := r.retry(ctx, func() (*client.Response, error) {
		return r.api.Get(ctx, key, opts)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Set assigns a new value to a Node identified by a given key. The caller
// may define a set of conditions in the SetOptions. If SetOptions.Dir=true
// then value is ignored.
func (r retryApi) Set(ctx context.Context, key, value string, opts *client.SetOptions) (*client.Response, error) {
	resp, err := r.retry(ctx, func() (*client.Response, error) {
		return r.api.Set(ctx, key, value, opts)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Delete removes a Node identified by the given key, optionally destroying
// all of its children as well. The caller may define a set of required
// conditions in an DeleteOptions object.
func (r retryApi) Delete(ctx context.Context, key string, opts *client.DeleteOptions) (*client.Response, error) {
	resp, err := r.retry(ctx, func() (*client.Response, error) {
		return r.api.Delete(ctx, key, opts)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Create is an alias for Set w/ PrevExist=false
func (r retryApi) Create(ctx context.Context, key, value string) (*client.Response, error) {
	resp, err := r.retry(ctx, func() (*client.Response, error) {
		return r.api.Create(ctx, key, value)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CreateInOrder is used to atomically create in-order keys within the given directory.
func (r retryApi) CreateInOrder(ctx context.Context, dir, value string, opts *client.CreateInOrderOptions) (*client.Response, error) {
	resp, err := r.retry(ctx, func() (*client.Response, error) {
		return r.api.CreateInOrder(ctx, dir, value, opts)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Update is an alias for Set w/ PrevExist=true
func (r retryApi) Update(ctx context.Context, key, value string) (*client.Response, error) {
	resp, err := r.retry(ctx, func() (*client.Response, error) {
		return r.api.Update(ctx, key, value)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Watcher builds a new Watcher targeted at a specific Node identified
// by the given key. The Watcher may be configured at creation time
// through a WatcherOptions object. The returned Watcher is designed
// to emit events that happen to a Node, and optionally to its children.
func (r retryApi) Watcher(key string, opts *client.WatcherOptions) client.Watcher {
	return r.api.Watcher(key, opts)
}

func (r retryApi) retry(ctx context.Context, fn apiCall) (resp *client.Response, err error) {
	interval := backoff.NewExponentialBackOff()
	interval.MaxElapsedTime = defaults.RetrySmallerMaxInterval
	if r.interval != 0 {
		interval.MaxElapsedTime = r.interval
	}
	b := backoff.WithContext(interval, ctx)
	err = backoff.Retry(func() (err error) {
		resp, err = fn()
		if utils.IsTransientClusterError(err) {
			log.WithField("err", trace.UserMessage(err)).Debug("Retry on transient etcd error.")
			return trace.Wrap(err)
		}
		if err != nil {
			return &backoff.PermanentError{Err: err}
		}
		return nil
	}, b)
	if err != nil {
		return nil, convertErr(err)
	}
	return resp, nil
}

type apiCall func() (*client.Response, error)

type retryApi struct {
	api      client.KeysAPI
	interval time.Duration
}
