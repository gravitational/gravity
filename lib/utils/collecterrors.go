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
			return nil, trace.Wrap(ctx.Err())
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
