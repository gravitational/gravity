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
	"time"

	. "gopkg.in/check.v1"
)

func (s *UtilsSuite) TestThrottleReceivesLastValue(c *C) {
	inCh := make(chan string)
	outCh := make(chan string)
	notifCh := make(chan string)
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go func(ctx context.Context, ch <-chan string, notifCh chan string, sentinel string) {
		for {
			select {
			case <-ctx.Done():
				return
			case value := <-ch:
				time.Sleep(100 * time.Millisecond)
				if value == sentinel {
					notifCh <- value
				}
			}
		}
	}(ctx, outCh, notifCh, "c")
	go ThrottlingPipe(ctx, inCh, outCh)
	for _, value := range []string{"a", "b", "c"} {
		inCh <- value
	}

	select {
	case value := <-notifCh:
		c.Assert(value, Equals, "c")
	case <-time.After(time.Second):
		c.Errorf("failed to wait for notification")
	}
}

func (s *UtilsSuite) TestThrottleDoesnotSendTheSameValue(c *C) {
	inCh := make(chan string)
	outCh := make(chan string)
	notifCh := make(chan string)
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go func(ctx context.Context, ch <-chan string, notifCh chan string) {
		for {
			select {
			case <-ctx.Done():
				return
			case value := <-ch:
				time.Sleep(100 * time.Millisecond)
				notifCh <- value
			}
		}
	}(ctx, outCh, notifCh)
	go ThrottlingPipe(ctx, inCh, outCh)
	inCh <- "a"
	input := <-notifCh
	c.Assert(input, Equals, "a")
	select {
	case input = <-notifCh:
		c.Errorf("unexpected %v received from long process", input)
	default:
	}
}
