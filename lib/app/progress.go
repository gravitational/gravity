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

package app

import (
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
)

// ImportState defines a set of application import states
type ImportState string

const (
	// ImportStatePullingImages defines the state of pulling docker images
	ImportStatePullingImages ImportState = "import_pulling_images"
	// ImportStateStoringRegistry defines the state of storing image layers into temporary registry
	ImportStateStoringRegistry ImportState = "import_storing_registry"
	// ImportStateCreatingPackage defines the state of creating an application package
	ImportStateCreatingPackage ImportState = "import_creating_package"
)

// State returns a text representation of this import state
func (r ImportState) State() string {
	return string(r)
}

// Completion returns a completion value of this import state
func (r ImportState) Completion() int {
	switch r {
	case ImportStatePullingImages:
		return 10
	case ImportStateStoringRegistry:
		return 40
	case ImportStateCreatingPackage:
		return 80
	default:
		return 0
	}
}

// Message returns user-friendly message about what's going on
func (r ImportState) Message() string {
	switch r {
	case ImportStatePullingImages:
		return "pulling images from registry"
	case ImportStateStoringRegistry:
		return "saving registry state"
	case ImportStateCreatingPackage:
		return "creating package with application"
	default:
		return ""
	}
}
func (r ImportState) String() string {
	return fmt.Sprintf("%s at %d%%", r.State(), r.Completion())
}

// ProgressSuccess defines a set of success states
type ProgressSuccess string

const (
	// ProgressStateInProgress defines an operation in progress
	ProgressStateInProgress ProgressSuccess = "in_progress"
	// ProgressStateCompleted defines a successful operation
	ProgressStateCompleted ProgressSuccess = "completed"
)

// State returns the state of this progress step
func (r ProgressSuccess) State() string {
	return string(r)
}

// Completion returns a completion value for this progress state
func (r ProgressSuccess) Completion() int {
	switch r {
	case ProgressStateCompleted:
		return constants.Completed
	default:
		return 0
	}
}

// Message returns a message associated with this progress state
func (r ProgressSuccess) Message() string { return "" }

// String returns a text representation of this progress state
func (r ProgressSuccess) String() string {
	switch r {
	case ProgressStateCompleted:
		return "operation complete"
	default:
		return "operation in progress"
	}
}

// ProgressFailure defines a set of failed states
type ProgressFailure string

// ProgressStateFailed is a final progress state that signifies a failed operation
var ProgressStateFailed ProgressFailure

// State returns the state of this progress step
func (r ProgressFailure) State() string {
	return "failed"
}

// Message returns an error message associated with this failure progress state
func (r ProgressFailure) Message() string {
	return string(r)
}

// String returns a text representation of this progress state
func (r ProgressFailure) String() string {
	return fmt.Sprintf("operation failed: %v", string(r))
}

// Completion returns a completion value for this progress state
func (r ProgressFailure) Completion() int {
	return constants.Completed
}
