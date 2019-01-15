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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Abort causes Retry function to stop with error
func Abort(err error) *AbortRetry {
	return &AbortRetry{Err: err}
}

// IsAbortError returns true if the specified error is of type AbortRetry
func IsAbortError(err error) bool {
	_, ok := trace.Unwrap(err).(*AbortRetry)
	return ok
}

// IsContinueError returns true if provided error is of ContinueRetry type
func IsContinueError(err error) bool {
	_, ok := trace.Unwrap(err).(*ContinueRetry)
	return ok
}

// Continue causes Retry function to continue trying and logging message
func Continue(format string, args ...interface{}) *ContinueRetry {
	return &ContinueRetry{Message: fmt.Sprintf(format, args...)}
}

// AbortRetry if returned from Retry, will lead to retries to be stopped,
// but the Retry function will return internal Error
type AbortRetry struct {
	Err error
}

// Error returns the abort error string representation
func (a *AbortRetry) Error() string {
	return fmt.Sprintf("Abort(%v)", a.Err)
}

// OriginalError returns the original error message this abort error wraps
func (a *AbortRetry) OriginalError() string {
	return a.Err.Error()
}

// ContinueRetry if returned from Retry, will be lead to retry next time
type ContinueRetry struct {
	Message string
}

// Error returns the continue error string representation
func (s *ContinueRetry) Error() string {
	return fmt.Sprintf("ContinueRetry(%v)", s.Message)
}

// Retry attempts to execute fn up to maxAttempts sleeping for period between attempts.
// fn can return an instance of Abort to abort or Continue to continue the execution.
func Retry(period time.Duration, maxAttempts int, fn func() error) error {
	var err error
	for i := 1; i <= maxAttempts; i += 1 {
		err = fn()
		if err == nil {
			return nil
		}
		switch origErr := err.(type) {
		case *AbortRetry:
			return origErr.Err
		case *ContinueRetry:
			log.Debugf("%v retry in %v.", origErr.Message, period)
		default:
			log.Debugf("Unsuccessful attempt %v/%v: %v, retry in %v.",
				i, maxAttempts, trace.UserMessage(err), period)
		}
		time.Sleep(period)
	}
	log.Errorf("All attempts failed:\n%v.", trace.DebugReport(err))
	return err
}

// RetryFor retries the provided function until it succeeds or until timeout has been reached
func RetryFor(ctx context.Context, timeout time.Duration, fn func() error) error {
	start := time.Now()
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if time.Now().Sub(start) > timeout {
			return trace.Wrap(err, "retry exceeded %v", timeout)
		}
		switch origErr := err.(type) {
		case *AbortRetry:
			return trace.Wrap(origErr.Err)
		case *ContinueRetry:
			log.Debugf("%v, will retry", origErr.Message)
		default:
			log.Debugf("%v, will retry", trace.UserMessage(err))
		}
		select {
		case <-time.After(defaults.RetryInterval):
		case <-ctx.Done():
			log.Infof("retry canceled")
			return trace.Wrap(err)
		}
	}
	return nil
}

// RetryRead reads the contents of the reader to the temporary file
// and retries several times on failure. It closes the reader returned
// by getReadCloser function at all times
func RetryRead(getReadCloser func() (io.ReadCloser, error), period time.Duration, attempts int) (io.ReadCloser, error) {
	var rc io.ReadCloser
	err := Retry(period, attempts, func() error {
		in, err := getReadCloser()
		if err != nil {
			return trace.Wrap(err)
		}
		defer in.Close()
		file, err := ioutil.TempFile("", "gravity-download")
		if err != nil {
			return trace.Wrap(err)
		}
		os.Remove(file.Name())
		_, err = io.Copy(file, in)
		if err != nil {
			file.Close()
			return trace.Wrap(err)
		}
		_, err = file.Seek(0, 0)
		if err != nil {
			file.Close()
			return trace.Wrap(err)
		}
		rc = file
		return nil
	})
	return rc, trace.Wrap(err)
}

// RetryOnNetworkError attempts to execute fn up to maxAttempts sleeping for period
// between attempts if the encountered error is of network nature.
func RetryOnNetworkError(period time.Duration, maxAttempts int, fn func() error) error {
	err := Retry(period, maxAttempts, func() error {
		err := fn()
		switch trace.Unwrap(err).(type) {
		case *net.OpError:
			return Continue("network error: %v", err)
		}
		if err != nil {
			return Abort(err)
		}
		return nil
	})
	return trace.Wrap(err)
}

// TeeReadCloser is just like TeeReader but implements io.ReadCloser
func TeeReadCloser(r io.ReadCloser, w io.Writer) io.ReadCloser {
	return &teeReadCloser{io.TeeReader(r, w), r, w}
}

type teeReadCloser struct {
	io.Reader
	rc io.ReadCloser
	w  io.Writer
}

func (t *teeReadCloser) Close() error {
	return t.rc.Close()
}

// RetryTransient retries the specified operation fn using the specified backoff interval
// if the operation is experiencing transient errors.
// Etcd cluster errors as well as kubernetes unauthorzied errors are considered transient.
// Returns any non-transient error or nil if the operation is successful.
func RetryTransient(ctx context.Context, interval backoff.BackOff, fn func() error) error {
	return trace.Wrap(RetryWithInterval(ctx, interval, func() error {
		err := fn()
		if err == nil {
			return nil
		}
		switch {
		case IsTransientClusterError(err):
			// Retry on transient etcd errors
			return trace.Wrap(err)
		case IsKubeAuthError(err):
			// Kubernetes replies with unauthorized for certain
			// operations when etcd is down
			return trace.Wrap(err)
		default:
			return &backoff.PermanentError{Err: err}
		}
	}))
}

// RetryWithInterval retries the specified operation fn using the specified backoff interval.
// classify specifies the error classifier that can create circuit-breakers for
// specific error conditions. classify should return backoff.PermanentError if the error
// should not be retried and returned directly.
// Returns nil on success or the last received error upon exhausting the interval.
func RetryWithInterval(ctx context.Context, interval backoff.BackOff, fn func() error) error {
	b := backoff.WithContext(interval, ctx)
	err := backoff.RetryNotify(func() (err error) {
		err = fn()
		return err
	}, b, func(err error, d time.Duration) {
		log.WithError(err).Infof("Retrying at %v.", d)
	})

	switch errOrig := trace.Unwrap(err).(type) {
	case *trace.RetryError:
		// TODO: fix trace.Retry.OrigError to return the original error
		err = errOrig.Err
	}
	if err != nil {
		log.Errorf("All attempts failed: %v.", trace.DebugReport(err))
		return trace.Wrap(err)
	}
	return nil
}

// NewUnlimitedExponentialBackOff returns a backoff interval without time restriction
func NewUnlimitedExponentialBackOff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0
	return b
}

// NewExponentialBackOff creates a new backoff interval with the specified timeout
func NewExponentialBackOff(timeout time.Duration) backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = timeout
	return b
}
