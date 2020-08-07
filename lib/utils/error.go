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
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cenkalti/backoff"
	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	return isEtcdClusterErrorMessage(trace.UserMessage(err))
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
	if err == nil {
		return false
	}

	switch {
	case trace.IsConnectionProblem(err):
		return true
	case IsConnectionResetError(err):
		return true
	case IsConnectionRefusedError(err):
		return true
	case IsClusterUnavailableError(err) || isEtcdClusterError(err):
		return true
	case isKubernetesEtcdClusterError(err):
		return true
	case IsKubeAuthError(err):
		// Kubernetes replies with unauthorized for certain
		// operations when etcd is down
		return true
	default:
		return false
	}
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
func NewUninstallServiceError(err error, servicePackage loc.Locator) error {
	return &ErrorUninstallService{
		Package: servicePackage,
		Err:     err,
	}
}

// Error implements error interface
func (r *ErrorUninstallService) Error() string {
	return fmt.Sprintf("failed uninstalling %v service: %v", r.Package, r.Err)
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

// IsResourceBusyError determines if the specified error identifies a 'device or resource busy' error
func IsResourceBusyError(err error) bool {
	switch err := trace.Unwrap(err).(type) {
	case *os.PathError:
		return isResourceBusyError(err.Err)
	default:
		return isResourceBusyError(err)
	}
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
	// Err specifies the actual error encountered while uninstalling
	// the service
	Err error
}

// IsPathError determines if the specified err is of type os.PathError
func IsPathError(err error) bool {
	_, ok := trace.Unwrap(err).(*os.PathError)
	return ok
}

func isResourceBusyError(err error) bool {
	sysErr, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	return sysErr == syscall.EBUSY
}

func isKubernetesEtcdClusterError(err error) bool {
	switch origErr := trace.Unwrap(err).(type) {
	case *kubeerrors.StatusError:
		return origErr.Status().Code == http.StatusInternalServerError &&
			isEtcdClusterErrorMessage(origErr.ErrStatus.Message)
	}
	return false
}

func isEtcdClusterError(err error) bool {
	_, ok := trace.Unwrap(err).(*etcd.ClusterError)
	return ok
}

func isEtcdClusterErrorMessage(message string) bool {
	return isEtcdClusterMisconfigured(message) ||
		isEtcdClusterHasNoLeader(message) ||
		isEtcdClusterLeaderChanged(message) ||
		isEtcdClusterRequestTimedOut(message)
}

func isEtcdClusterMisconfigured(message string) bool {
	return strings.Contains(message, "etcd cluster is unavailable or misconfigured")
}

func isEtcdClusterHasNoLeader(message string) bool {
	return strings.Contains(message, "etcd member") &&
		strings.Contains(message, "has no leader")
}

func isEtcdClusterLeaderChanged(message string) bool {
	return strings.Contains(message, "etcdserver: leader changed")
}

func isEtcdClusterRequestTimedOut(message string) bool {
	return strings.Contains(message, "etcdserver: request timed out")
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

// ConvertEC2Error converts error from AWS EC2 API to appropriate trace error.
func ConvertEC2Error(err error) error {
	if err == nil {
		return nil
	}
	awsErr, ok := err.(awserr.Error)
	if !ok {
		return err
	}
	// For some reason, AWS Go SDK does not define constants for EC2 error
	// codes so we're using strings here.
	switch awsErr.Code() {
	case "InvalidInstanceID.NotFound":
		return trace.NotFound(awsErr.Message())
	case "InvalidInstanceID.Malformed":
		return trace.BadParameter(awsErr.Message())
	}
	return err
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

// IsConnectionResetError determines whether err is a
// 'connection reset by peer' error.
// err is expected to be non-nil
func IsConnectionResetError(err error) bool {
	return strings.Contains(trace.Unwrap(err).Error(),
		"connection reset by peer")
}

// IsConnectionRefusedError determines whether err is a
// 'connection refused' error.
// err is expected to be non-nil
func IsConnectionRefusedError(err error) bool {
	if urlError, ok := trace.Unwrap(err).(*url.Error); ok {
		if opError, ok := urlError.Err.(*net.OpError); ok {
			errno, ok := opError.Err.(syscall.Errno)
			return ok && errno == syscall.ECONNREFUSED
		}
	}
	return strings.Contains(trace.Unwrap(err).Error(),
		"connection refused")
}

// ShouldReconnectPeer implements the error classification for peer connection errors
//
// It detects unrecoverable errors and aborts the reconnect attempts
func ShouldReconnectPeer(err error) error {
	switch {
	case isPermissionDeniedError(err),
		isLicenseError(err.Error()),
		isHostAlreadyRegisteredError(err.Error()):
		return &backoff.PermanentError{Err: err}
	}
	return err
}

// NewFailedPreconditionError returns a new failed precondition error
// with optional original error err
func NewFailedPreconditionError(err error) error {
	return WrapExitCodeError(defaults.FailedPreconditionExitCode, err)
}

// ExitCodeError defines an interface for exit code errors
type ExitCodeError interface {
	error
	// ExitCode returns the numeric error code to exit with
	ExitCode() int
	// OrigError returns the original error this error wraps.
	OrigError() error
}

// NewPreconditionFailedError returns a new error signifying a failed
// precondition
func NewPreconditionFailedError(err error) error {
	return exitCodeError{
		code: defaults.FailedPreconditionExitCode,
		err:  err,
	}
}

// NewExitCodeError returns a new error with the specified exit code
func NewExitCodeError(exitCode int) error {
	if exitCode == 0 {
		return nil
	}
	return exitCodeError{code: exitCode}
}

// NewExitCodeError returns a new error that wraps a specific exit code and message
func NewExitCodeErrorWithMessage(exitCode int, message string) error {
	return exitCodeError{
		code:    exitCode,
		message: message,
	}
}

// WrapExitCodeError returns a new error with the specified exit code
// that wrap another error
func WrapExitCodeError(exitCode int, err error) error {
	return exitCodeError{
		code: exitCode,
		err:  err,
	}
}

// ExitStatusFromError returns the exit status from the specified error.
// If the error is not exit status error, return nil
func ExitStatusFromError(err error) *int {
	exitErr, ok := trace.Unwrap(err).(*exec.ExitError)
	if !ok {
		return nil
	}
	if waitStatus, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok {
		status := waitStatus.ExitStatus()
		return &status
	}
	return nil
}

// IsCompareFailedError returns true to indicate this error
// complies with compare failed error protocol
func (ClusterDegradedError) IsCompareFailedError() bool {
	return true
}

// Error returns the text representation of this error
func (ClusterDegradedError) Error() string {
	return "cluster is degraded"
}

// ClusterDegradedError indicates that the cluster is degraded
type ClusterDegradedError struct{}

// ExitCode interprets this value as exit code.
// Implements ExitCodeError
func (r exitCodeError) ExitCode() int {
	return r.code
}

// IsClusterDegradedError determines if the error indicates that
// the cluster is degraded
func IsClusterDegradedError(err error) bool {
	if _, ok := trace.Unwrap(err).(ClusterDegradedError); ok {
		return true
	}
	// Handle the case when the error has come over the wire
	return strings.Contains(err.Error(), "cluster is degraded")
}

// Error returns this exit code as error string.
// Implements error
func (r exitCodeError) Error() string {
	if r.err != nil {
		return r.err.Error()
	}
	if r.message != "" {
		return r.message
	}
	return fmt.Sprintf("exit with code %v", r.code)
}

// OrigError returns the original error if available.
// If no error has been stored, it returns this error
func (r exitCodeError) OrigError() error {
	if r.err == nil {
		return r
	}
	return r.err
}

type exitCodeError struct {
	code    int
	message string
	// err specifies optional original error
	err error
}

func isPermissionDeniedError(err error) bool {
	if err == nil {
		return false
	}
	if statusErr, ok := status.FromError(trace.Unwrap(err)); ok {
		return statusErr.Code() == codes.PermissionDenied
	}
	return trace.IsAccessDenied(err)
}

func isLicenseError(message string) bool {
	return strings.Contains(message, "license allows maximum of")
}

func isHostAlreadyRegisteredError(message string) bool {
	return strings.Contains(message, "One of existing peers already has hostname") ||
		strings.Contains(message, "One of existing servers already has hostname")
}
