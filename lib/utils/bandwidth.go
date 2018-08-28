package utils

import (
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	log "github.com/sirupsen/logrus"
	"github.com/codahale/hdrhistogram"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
)

// BandwidthWriter is a writer that calculates amount of traffic (bytes)
// going through it
type BandwidthWriter struct {
	sync.Mutex
	// bytes is the total amount of bytes written in the current second
	bytes uint64
	// histogram is the HDR histogram that records byte values every second
	histogram *hdrhistogram.Histogram
	// clock is used in tests to mock time
	clock timetools.TimeProvider
	// closeCh is the close channel
	closeCh chan struct{}
}

// NewBandwidthWriter creates a new writer that calculates its traffic bandwidth
//
// Writer needs to be closed after it is no longer needed to prevent leaking
// goroutines
func NewBandwidthWriter() *BandwidthWriter {
	w := &BandwidthWriter{
		histogram: hdrhistogram.New(0, defaults.BandwidthMaxSpeedBytes, 5),
		clock:     &timetools.RealTime{},
		closeCh:   make(chan struct{}),
	}
	go w.start()
	return w
}

// Write adds the amount of provided bytes to the current second's total
func (w *BandwidthWriter) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()

	w.bytes += uint64(len(p))
	return len(p), nil
}

// Close stops the writer's goroutine
func (w *BandwidthWriter) Close() error {
	close(w.closeCh)
	return nil
}

// Max returns the maximum recorded value
func (w *BandwidthWriter) Max() uint64 {
	return uint64(w.histogram.Max())
}

// start launches bandwidth calculation every second
func (w *BandwidthWriter) start() {
	for {
		select {
		case <-w.clock.After(time.Second):
			err := w.tick()
			if err != nil {
				log.Error(trace.DebugReport(err))
			}
		case <-w.closeCh:
			log.Info("closing bandwidth writer")
			return
		}
	}
}

// tick calculates the current and maximum bandwidth based on the recorded
// amount of currently processed bytes
func (w *BandwidthWriter) tick() error {
	w.Lock()
	defer func() {
		// reset the bytes
		w.bytes = 0
		w.Unlock()
	}()

	// record the current value in the histogram
	return trace.Wrap(w.histogram.RecordValue(int64(w.bytes)))
}
