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

// IsResume determines if this request describes a resume operation
func (r *ExecuteRequest) IsResume() bool {
	return r.Phase == nil || r.Phase.IsResume()
}

// IsResume determines if this phase describes a resume operation
func (r *ExecuteRequest_Phase) IsResume() bool {
	return r.ID == fsm.RootPhase
}

// Empty defines the empty RPC message
var Empty = &types.Empty{}

// IsAbortError returns true if the specifies error identifies the aborted operation
func IsAbortError(err error) bool {
	return trace.Unwrap(err) == ErrAborted
}

// ErrAborted defines the aborted operation error
var ErrAborted = utils.NewExitCodeErrorWithMessage(defaults.AbortedOperationExitCode, "operation aborted")

// AbortEvent is a progress response that indicates an aborted operation
var AbortEvent = &ProgressResponse{
	Status: StatusAborted,
}
