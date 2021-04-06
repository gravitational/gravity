/*
Copyright 2016 Gravitational, Inc.

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

package client

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	grpcerrors "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
		return status.Error(grpcerrors.Unknown, "unknown error")
	}
}
