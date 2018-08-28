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
		return &backoff.PermanentError{Err: origErr}
	}
}
