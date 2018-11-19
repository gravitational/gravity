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
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/gravity/lib/loc"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
)

// ToError either returns error as is, or converts it to Errorf
// in case of unknown object
func ToError(i interface{}) error {
	err, ok := i.(error)
	if ok {
		return err
	}
	return trace.Errorf("unrecognized error: %#v", i)
}

// ToRawTrace converts the trace error to marshable format
func ToRawTrace(err trace.Error) *trace.RawTrace {
	if err == nil {
		return nil
	}
	result := &trace.RawTrace{}
	if traceErr, ok := err.(*trace.TraceErr); ok {
		result.Traces = traceErr.Traces
		result.Message = traceErr.Message
	}
	bytes, errMarshal := json.Marshal(message{err.OrigError().Error()})
	if errMarshal != nil {
		bytes = []byte(errMarshal.Error())
	}
	result.Err = json.RawMessage(bytes)
	return result
}

// UnmarshalError unmarshals bytes as JSON-encoded error
func UnmarshalError(bytes []byte, err *trace.TraceErr) error {
	if len(bytes) == 0 {
		return nil
	}
	var message message
	if errUnmarshal := json.Unmarshal(bytes, &message); errUnmarshal != nil {
		return trace.Wrap(errUnmarshal)
	}
	err.Err = message
	return nil
}

// IsClosedConnectionError determines if the specified error is a closed connection error
func IsClosedConnectionError(err error) bool {
	text := trace.Unwrap(err).Error()
	return strings.Contains(text, "use of closed network connection")
}

// IsClusterUnavailableError determines if the specified error is a cluster unavailable error
func IsClusterUnavailableError(err error) bool {
	text := trace.Unwrap(err).Error()
	return isEtcdClusterError(text)
}

// IsKubeAuthError determines whether the specified error is an authorization
// error from kubernetes client.
func IsKubeAuthError(err error) bool {
	if statusErr, ok := trace.Unwrap(err).(*kubeerrors.StatusError); ok {
		return statusErr.Status().Code == http.StatusUnauthorized
	}
	return false
}

// IsTransientClusterError determines if the specified error corresponds to a transient
// error - e.g. which can be retried. An error that can be retried
// is either a connection failure or an etcd cluster error.
func IsTransientClusterError(err error) bool {
	if trace.IsConnectionProblem(err) {
		return true
	}

	switch origErr := trace.Unwrap(err).(type) {
	case *etcd.ClusterError:
		return true
	case *kubeerrors.StatusError:
		if origErr.Status().Code == http.StatusInternalServerError && isEtcdClusterError(origErr.ErrStatus.Message) {
			return true
		}
	}
	return err != nil && isEtcdClusterError(err.Error())
}

// IsNetworkError returns true if the provided error is Go's network error
func IsNetworkError(err error) bool {
	switch trace.Unwrap(err).(type) {
	case *net.OpError:
		return true
	}
	return false
}

// NewUninstallServiceError returns a plan out of sync error
func NewUninstallServiceError(servicePackage loc.Locator) error {
	return &ErrorUninstallService{Package: servicePackage}
}

// Error implements error interface
func (r *ErrorUninstallService) Error() string {
	return fmt.Sprintf("failed uninstalling %v service", r.Package)
}

// IsStreamClosedError determines if the given error is a response/stream closed
// error
func IsStreamClosedError(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case trace.Unwrap(err) == io.EOF:
		return true
	case IsClosedResponseBodyErrorMessage(err.Error()):
		return true
	}
	return false
}

// IsClosedResponseBodyErrorMessage determines if the error message
// describes a closed response body error
func IsClosedResponseBodyErrorMessage(err string) bool {
	return strings.HasSuffix(err, "response body closed")
}

// ErrorUninstallService is an error returned for failed service uninstall attempts
type ErrorUninstallService struct {
	// Package refers to the service that failed to uninstall
	Package loc.Locator
}

// IsPathError determines if the specified err is of type os.PathError
func IsPathError(err error) bool {
	_, ok := trace.Unwrap(err).(*os.PathError)
	return ok
}

func isEtcdClusterError(message string) bool {
	return isEtcdClusterMisconfigured(message) || isEtcdClusterHasNoLeader(message)
}

func isEtcdClusterMisconfigured(message string) bool {
	return strings.Contains(message, "etcd cluster is unavailable or misconfigured")
}

func isEtcdClusterHasNoLeader(message string) bool {
	return strings.Contains(message, "etcd member") &&
		strings.Contains(message, "has no leader")
}

// MarshalJSON marshals this message as JSON.
// Implements json.Marshaler
func (r message) MarshalJSON() (bytes []byte, err error) {
	type msg message
	bytes, err = json.Marshal(&msg{r.Error()})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes, nil
}

// UnmarshalJSON unmarshals a message from JSON-encoded bytes
// Implements json.UnMarshaler
func (r *message) UnmarshalJSON(bytes []byte) error {
	type msg message
	var result msg
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return trace.Wrap(err)
	}
	*r = message(result)
	return nil
}

// Error returns underlying message.
// Implements error
func (r message) Error() string {
	return r.Message
}

// message is a transport-friendly error
type message struct {
	// Message is the error message
	Message string `json:"message"`
}

// ConvertS3Error converts an error from AWS S3 API to an appropriate trace error
func ConvertS3Error(err error) error {
	if err == nil {
		return nil
	}
	awsErr, ok := err.(awserr.Error)
	if !ok {
		return err
	}
	switch awsErr.Code() {
	case s3.ErrCodeNoSuchKey, s3.ErrCodeNoSuchBucket:
		return trace.NotFound(awsErr.Message())
	}
	return err
}

// UnsupportedFilesystemError represents a condition when an action is being
// performed on an unsupported filesystem, for example an attempt to create
// a bolt database file on filesystem that does not support mmap
type UnsupportedFilesystemError struct {
	// Err is the original error
	Err error
	// Path is path to the directory with unsupported filesystem
	Path string
}

// Error returns the string representation of the error
func (e *UnsupportedFilesystemError) Error() string {
	return e.Err.Error()
}

// NewUnsupportedFilesystemError creates a new error for an unsupported filesystem at the specified path
func NewUnsupportedFilesystemError(err error, path string) *UnsupportedFilesystemError {
	return &UnsupportedFilesystemError{Err: err, Path: path}
}

// IsContextCancelledError returns true if the provided error is a result
// of a context cancellation
func IsContextCancelledError(err error) bool {
	origErr := trace.Unwrap(err)
	if origErr == context.Canceled {
		return true
	}
	if connErr, ok := origErr.(*trace.ConnectionProblemError); ok {
		return connErr.Err == context.Canceled
	}
	return false
}

// ShouldReconnectPeer implements the error classification for peer connection errors
//
// It detects unrecoverable errors and aborts the reconnect attempts
func ShouldReconnectPeer(err error) error {
	if isPeerDeniedError(err.Error()) {
		return &backoff.PermanentError{err}
	}
	return err
}

// IsConnectionResetError determines whether err is a
// 'connection reset by peer' error.
// err is expected to be non-nil
func IsConnectionResetError(err error) bool {
	return strings.Contains(trace.Unwrap(err).Error(),
		"connection reset by peer")
}

func isPeerDeniedError(message string) bool {
	return strings.Contains(message, "AccessDenied")
}
