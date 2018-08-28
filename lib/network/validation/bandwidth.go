package validation

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	pb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// CheckBandwidth launches a special type of a ping-pong game that measures network
// bandwidth between servers specified in the request
func (r *Server) CheckBandwidth(ctx context.Context, req *pb.CheckBandwidthRequest) (*pb.CheckBandwidthResponse, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	duration, err := pb.DurationFromProto(req.Duration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var remoteIPs []string
	for _, ping := range req.Ping {
		remoteIPs = append(remoteIPs, ping.Addr)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%v:%v", req.Listen.Addr, defaults.BandwidthTestPort))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer listener.Close()

	bandwidth, err := checkBandwidth(listener, remoteIPs, duration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.CheckBandwidthResponse{Bandwidth: bandwidth}, nil
}

// checkBandwidth starts a server that listens for incoming data from other game
// participants and at the same time sends data to them, and calculates the amount
// of sent/received bytes
func checkBandwidth(listener net.Listener, remoteIPs []string, duration time.Duration) (uint64, error) {
	log.Info("Started bandwidth listener.")

	w := utils.NewBandwidthWriter()
	defer w.Close()

	// collect errors from server/clients in this channel
	errCh := make(chan error, len(remoteIPs)+1)

	deadline := time.Now().Add(duration)

	// start server
	go func() {
		errCh <- trace.Wrap(serve(listener, deadline, w))
	}()

	// start clients
	for _, server := range remoteIPs {
		go func(server string) {
			errCh <- trace.Wrap(startSendingData(server, deadline, w))
		}(server)
	}

	// let the test run for the specified duration, it will
	// get aborted after that, or until the first error
	select {
	case err := <-errCh:
		if err != nil {
			return 0, trace.Wrap(err)
		}
	case <-time.After(duration):
		break
	}

	log.Infof("bandwidth: %vB/s", w.Max())
	return w.Max(), nil
}

// serve starts a server that discards incoming data
// but uses "bandwidth writer" to calculate number of received bytes
func serve(listener net.Listener, deadline time.Time, w *utils.BandwidthWriter) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return trace.Wrap(err)
		}

		err = conn.SetDeadline(deadline)
		if err != nil {
			return trace.Wrap(err)
		}

		go func(conn net.Conn) {
			defer conn.Close()
			// received data is discarded by bandwidth writer, it just
			// calculates amount of bytes
			_, err := io.Copy(w, conn)
			// because there is no definitive amount of data to receive - just
			// continuous stream of data - copy is expected to end with error
			// once the test duration has expired and listener/connections are
			// aborted, so we're not treating it as a failure, but log anyway
			if err != nil && !utils.IsStreamClosedError(err) {
				log.Error(trace.DebugReport(err))
			}
		}(conn)
	}
}

// startSendingData opens a connection to a remote listener and submits
// bogus data to it while counting amount of sent bytes
func startSendingData(server string, deadline time.Time, w *utils.BandwidthWriter) error {
	var conn net.Conn
	var err error
	addr := fmt.Sprintf("%v:%v", server, defaults.BandwidthTestPort)
	// try connecting to remote servers a few times
	// as they may still be starting up
	err = utils.Retry(time.Second, 4, func() error {
		conn, err = net.Dial("tcp", addr)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	defer conn.Close()
	err = conn.SetDeadline(deadline)
	if err != nil {
		return trace.Wrap(err)
	}

	// use /dev/zero as a continuous source of bogus data,
	// it doesn't matter what we send really as it gets
	// discarded on the other end anyway
	reader, err := os.Open("/dev/zero")
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	_, err = io.Copy(io.MultiWriter(conn, w), reader)
	// because there is no definitive amount of data to receive - just
	// continuous stream of data - copy is expected to end with error
	// once the test duration has expired and listener/connections are
	// aborted, so we're not treating it as a failure, but log anyway
	if err != nil && !utils.IsStreamClosedError(err) {
		log.Error(trace.DebugReport(err))
	}

	return nil
}
