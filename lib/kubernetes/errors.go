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

package kubernetes

import (
	"github.com/cenkalti/backoff"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/api/errors"
)

// RetryOnUpdateConflict retries on update conflict errors
func RetryOnUpdateConflict(err error) error {
	if err == nil {
		return nil
	}
	origErr := trace.Unwrap(err)
	switch {
	case errors.IsConflict(origErr):
		return rigging.ConvertError(origErr)
	default:
		return &backoff.PermanentError{Err: err}
	}
}
