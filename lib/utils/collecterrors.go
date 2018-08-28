package utils

import (
	"context"

	"github.com/gravitational/trace"
)

// CollectErrors exhausts error channel errChan up to its capacity and returns aggregate error if any
func CollectErrors(ctx context.Context, errChan chan error) error {
	_, err := Collect(ctx, nil, errChan, nil)
	return trace.Wrap(err)
}

// Collect collects errors and values from channel provided, honouring timeout
// it will expect exactly cap(errChan) messages
// value channel could be nil. If not nil, then cap(errCh) == cap(valueCh)
// it will also cancel context on first error occured if cancel func is not nil
func Collect(ctx context.Context, cancel func(), errChan chan error, valuesChan chan interface{}) ([]interface{}, error) {
	errors := []error{}
	values := []interface{}{}

	errorsLeft := cap(errChan)
	valuesLeft := cap(valuesChan)

	if valuesLeft != 0 && (errorsLeft != valuesLeft) {
		return nil, trace.Errorf("cap(errChan)=%d, cap(valueChan)=%d", errorsLeft, valuesLeft)
	}

	for errorsLeft > 0 || valuesLeft > 0 {
		select {
		case <-ctx.Done():
			errors = append(errors, trace.Errorf("timed out"))
			return nil, trace.NewAggregate(errors...)
		case err := <-errChan:
			errorsLeft--
			if err != nil {
				errors = append(errors, err)
				if cancel != nil {
					cancel()
				}
			}
		case val := <-valuesChan:
			valuesLeft--
			values = append(values, val)
		}
	}

	return values, trace.NewAggregate(errors...)
}
