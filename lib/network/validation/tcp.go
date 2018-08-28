package validation

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// listenTCP starts a TCP server that listens on the provided address
// for the specified duration and then stops
func listenTCP(ctx context.Context, address string, duration time.Duration) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("started TCP listener: %v", address)
	ctx, cancel := context.WithTimeout(ctx, duration)
	go func() {
		<-ctx.Done()
		cancel()
		listener.Close()
	}()
Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		default:
			err := handleTCPConnections(listener, duration)
			if err != nil {
				log.Errorf(trace.DebugReport(err))
			}
		}
	}
	log.Debugf("stopped TCP listener: %v", address)
	return nil
}

// handleTCPConnections waits for the next connection on the provided listener
// and replies with "pong"
func handleTCPConnections(listener net.Listener, duration time.Duration) error {
	conn, err := listener.Accept()
	if err != nil {
		if utils.IsClosedConnectionError(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(duration))
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("received tcp ping: %v -> %v", conn.RemoteAddr(), conn.LocalAddr())
	_, err = io.WriteString(conn, "pong\n")
	return trace.Wrap(err)
}

// pingTCP connects to the specified TCP server, sends "ping" request
// and waits for the "pong" response for the specified duration
func pingTCP(address string, duration time.Duration) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(duration))
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = io.WriteString(conn, "ping\n")
	if err != nil {
		return trace.Wrap(err)
	}
	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return trace.Wrap(err)
	}
	if strings.TrimSpace(response) != "pong" {
		return trace.BadParameter("unexpected response: %v", response)
	}
	return nil
}
