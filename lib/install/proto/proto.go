/*
Copyright 2019 Gravitational, Inc.

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
package installer

import (
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsAborted returns true if this progress response indicates an aborted operation
func (r *ProgressResponse) IsAborted() bool {
	return r.Status == StatusAborted
}

// IsCompleted returns true if this progress response indicates a completed operation
func (r *ProgressResponse) IsCompleted() bool {
	return r.Status == StatusCompleted ||
		r.Status == StatusCompletedPending
}

// String formats this status as text
func (r ProgressResponse_Status) String() string {
	switch r {
	case StatusCompleted:
		return "completed"
	case StatusCompletedPending:
		return "pending"
	}
	return "unknown"
}

// KeyFromProto converts the specified operation key to internal format
func KeyFromProto(key *OperationKey) ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   key.AccountID,
		SiteDomain:  key.ClusterName,
		OperationID: key.ID,
	}
}

// KeyToProto converts the specified operation key to proto format
func KeyToProto(key ops.SiteOperationKey) *OperationKey {
	return &OperationKey{
		AccountID:   key.AccountID,
		ClusterName: key.SiteDomain,
		ID:          key.OperationID,
	}
}

// HasSpecificPhase determines if this request is for a specific phase (other than root)
func (r *ExecuteRequest) HasSpecificPhase() bool {
	return !r.HasResume()
}

// HasResume determines if this is a request to resume an operation
func (r *ExecuteRequest) HasResume() bool {
	return r.Phase == nil || r.Phase.IsResume()
}

// OperationKey returns operation key from request.
func (r *SetStateRequest) OperationKey() ops.SiteOperationKey {
	return KeyFromProto(r.Phase.Key)
}

// OperationID returns operation ID from request.
func (r *SetStateRequest) OperationID() string {
	return r.Phase.ID
}

// IsResume determines if this phase describes a resume operation
func (r *Phase) IsResume() bool {
	return r.ID == fsm.RootPhase
}

// WrapServiceError returns an error from service optionally
// translating it to a more appropriate representation if required
func WrapServiceError(err error) error {
	if isErrorCode(err, codes.FailedPrecondition, codes.PermissionDenied) {
		return utils.NewExitCodeErrorWithMessage(
			defaults.FailedPreconditionExitCode,
			trace.UserMessage(err),
		)
	}
	return trace.Wrap(err)
}

// Empty defines the empty RPC message
var Empty = &types.Empty{}

// IsAbortedError returns true if the specified error identifies the aborted operation
func IsAbortedError(err error) bool {
	return trace.Unwrap(err) == ErrAborted
}

// IsCompletedError returns true if the specified error identifies the completed operation
func IsCompletedError(err error) bool {
	return trace.Unwrap(err) == ErrCompleted
}

// IsRPCError returns true if the specified error is a gRPC error
func IsRPCError(err error) bool {
	_, ok := status.FromError(trace.Unwrap(err))
	return ok
}

// IsFailedPreconditionError returns true if the specified error indicates a failed precondition
// RPC error
func IsFailedPreconditionError(err error) bool {
	s, ok := status.FromError(trace.Unwrap(err))
	return ok && s.Code() == codes.FailedPrecondition
}

// ErrAborted defines the aborted operation error
var ErrAborted = utils.NewExitCodeErrorWithMessage(defaults.AbortedOperationExitCode, "operation aborted")

// ErrCompleted defines the completed operation error.
// This is not an error in the usual sense - rather, it indicates that the operation
// has been completed and that the agent should shut down and not restart
var ErrCompleted = utils.NewExitCodeErrorWithMessage(defaults.CompletedOperationExitCode, "operation completed")

// AbortEvent is a progress response that indicates an aborted operation
var AbortEvent = &ProgressResponse{
	Status: StatusAborted,
}

// CompleteEvent is a progress response that indicates a successfully completed operation
var CompleteEvent = &ProgressResponse{
	Status: StatusCompleted,
}

func isErrorCode(err error, codes ...codes.Code) bool {
	s, ok := status.FromError(trace.Unwrap(err))
	if !ok {
		return false
	}
	for _, c := range codes {
		if s.Code() == c {
			return true
		}
	}
	return false
}
