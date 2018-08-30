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

import "context"

// ThrottlingPipe connects a producer writing to inCh
// with a consumer reading from outCh.
// The function matches the consumption rate of outCh
// by dropping all values but the last one it receives from inCh.
// The last value is always guaranteed to be sent to consumer.
func ThrottlingPipe(ctx context.Context, inCh <-chan string, outCh chan string) {
	// consumerCh aliases outCh and is active only when there's a new value
	// for the consumer
	var consumerCh chan string
	var lastValue string
	for {
		select {
		case lastValue = <-inCh:
			// Keep the last value and re-enable the consumer channel.
			// If the consumer cannot immediately receive a value on the
			// next iteration (e.g. when it is busy), this will block until
			// either another value is received from input channel or the
			// consumer becomes available
			consumerCh = outCh
		case consumerCh <- lastValue:
			// Send the last value we have so far and disable the consumer.
			// The consumer will automatically be enabled upon receiving the
			// next value.
			consumerCh = nil
		case <-ctx.Done():
			return
		}
	}
}
