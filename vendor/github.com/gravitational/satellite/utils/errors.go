/*
Copyright 2020 Gravitational, Inc.

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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	grpcerrors "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConvertError converts the err into a proper trace error.
func ConvertError(err error) error {
	return ConvertErrorWithContext(err, "")
}

// ConvertErrorWithContext converts the err into a proper trace error.
func ConvertErrorWithContext(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	statusErr, ok := err.(*errors.StatusError)
	if !ok {
		return err
	}

	message := fmt.Sprintf("%v", err)
	if !isEmptyDetails(statusErr.ErrStatus.Details) {
		message = fmt.Sprintf("%v, details: %v", message, statusErr.ErrStatus.Details)
	}
	if format != "" {
		message = fmt.Sprintf("%v: %v", fmt.Sprintf(format, args...), message)
	}

	status := statusErr.Status()
	switch {
	case status.Code == http.StatusConflict && status.Reason == metav1.StatusReasonAlreadyExists:
		return trace.AlreadyExists(message)
	case status.Code == http.StatusNotFound:
		return trace.NotFound(message)
	case status.Code == http.StatusForbidden:
		return trace.AccessDenied(message)
	}
	return err
}

// ConvertGRPCError maps grpc error to one of trace type classes.
// Returns original error if no mapping is possible
func ConvertGRPCError(err error) error {
	if err == nil {
		return nil
	}

	switch grpc.Code(trace.Unwrap(err)) {
	case grpcerrors.InvalidArgument, grpcerrors.OutOfRange:
		return trace.BadParameter(err.Error())
	case grpcerrors.DeadlineExceeded:
		return trace.LimitExceeded(err.Error())
	case grpcerrors.AlreadyExists:
		return trace.AlreadyExists(err.Error())
	case grpcerrors.NotFound:
		return trace.NotFound(err.Error())
	case grpcerrors.PermissionDenied, grpcerrors.Unauthenticated:
		return trace.AccessDenied(err.Error())
	case grpcerrors.Unimplemented:
		return trace.NotImplemented(err.Error())
	}
	return trace.Wrap(err)
}

// IsUnavailableError determines if the specified error
// is a temporary agent availability error
func IsUnavailableError(err error) bool {
	err = ConvertGRPCError(err)
	switch {
	case grpc.Code(trace.Unwrap(err)) == grpcerrors.Unavailable:
		return true
	case trace.IsLimitExceeded(err):
		return true
	}
	return false
}

// GRPCError converts the provided error into a grpc error.
// TODO: Define additional errors
func GRPCError(err error) error {
	switch trace.Unwrap(err) {
	case context.Canceled:
		return status.Error(grpcerrors.Canceled, "rpc canceled")
	case context.DeadlineExceeded:
		return status.Error(grpcerrors.DeadlineExceeded, "rpc deadline exceeded")
	default:
		logrus.WithError(err).Warn("Failed to convert to grpc error.")
		return status.Error(grpcerrors.Unknown, "unknown error")
	}
}

// IsContextCanceledError determines if the error indicates a canceled context
func IsContextCanceledError(err error) bool {
	return trace.Unwrap(err) == context.Canceled
}

func isEmptyDetails(details *metav1.StatusDetails) bool {
	if details == nil {
		return true
	}

	if details.Name == "" && details.Group == "" && details.Kind == "" && len(details.Causes) == 0 {
		return true
	}
	return false
}
