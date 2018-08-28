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

package monitoring

import (
	"context"

	"github.com/gravitational/satellite/agent/health"
)

// compositeChecker defines a health.Checker as a composite of
// several checkers run as a whole
type compositeChecker struct {
	name     string
	checkers []health.Checker
}

// Name returns the name of this checker
func (r *compositeChecker) Name() string { return r.name }

// Check runs an health check over the list of encapsulated checkers
// and reports errors to the specified Reporter
func (r *compositeChecker) Check(ctx context.Context, reporter health.Reporter) {
	for _, checker := range r.checkers {
		checker.Check(ctx, reporter)
	}
}

// NewCompositeChecker makes checker out of array of checkers
func NewCompositeChecker(name string, checkers []health.Checker) health.Checker {
	return &compositeChecker{name, checkers}
}
