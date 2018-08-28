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
