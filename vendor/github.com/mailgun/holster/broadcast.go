package holster

import "sync"

type Broadcaster interface {
	WaitChan(string) chan struct{}
	Wait(string)
	Broadcast()
	Done()
}

// Broadcasts to goroutines a new event has occurred and any waiting go routines should
// stop waiting and do work. The current implementation is limited to 10,0000 unconsumed
// broadcasts. If the user broadcasts more events than can be consumed calls to broadcast()
// will eventually block until the goroutines can catch up. This ensures goroutines will
// receive at least one event per broadcast() call.
type broadcast struct {
	clients map[string]chan struct{}
	done    chan struct{}
	mutex   sync.Mutex
}

func NewBroadcaster() Broadcaster {
	return &broadcast{
		clients: make(map[string]chan struct{}),
		done:    make(chan struct{}),
	}
}

// Notify all Waiting goroutines
func (b *broadcast) Broadcast() {
	b.mutex.Lock()
	for _, channel := range b.clients {
		channel <- struct{}{}
	}
	b.mutex.Unlock()
}

// Cancels any Wait() calls that are currently blocked
func (b *broadcast) Done() {
	close(b.done)
}

// Blocks until a broadcast is received
func (b *broadcast) Wait(name string) {
	b.mutex.Lock()
	channel, ok := b.clients[name]
	if !ok {
		b.clients[name] = make(chan struct{}, 10000)
		channel = b.clients[name]
	}
	b.mutex.Unlock()

	// Wait for a new event or done is closed
	select {
	case <-channel:
		return
	case <-b.done:
		return
	}
}

// Returns a channel the caller can use to wait for a broadcast
func (b *broadcast) WaitChan(name string) chan struct{} {
	b.mutex.Lock()
	channel, ok := b.clients[name]
	if !ok {
		b.clients[name] = make(chan struct{}, 10000)
		channel = b.clients[name]
	}
	b.mutex.Unlock()
	return channel
}
