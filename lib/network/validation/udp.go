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

package validation

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// listenUDP starts a UDP server that listens on the provided address
// for the specified duration and then stops
func listenUDP(ctx context.Context, address string, duration time.Duration) error {
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("started UDP listener: %v", address)
	ctx, cancel := context.WithTimeout(ctx, duration)
	go func() {
		<-ctx.Done()
		cancel()
		conn.Close()
	}()
Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		default:
			err := handleUDPPacket(conn)
			if err != nil {
				log.Errorf(trace.DebugReport(err))
			}
		}
	}
	log.Debugf("stopped UDP listener: %v", address)
	return nil
}

// handleUDPPacket waits for the next UDP packet and replies
// with "pong"
func handleUDPPacket(conn net.PacketConn) error {
	buf := make([]byte, 4)
	_, raddr, err := conn.ReadFrom(buf)
	if err != nil {
		if utils.IsClosedConnectionError(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	log.Infof("received udp ping: %v -> %v", raddr, conn.LocalAddr())
	_, err = conn.WriteTo([]byte("pong"), raddr)
	return trace.Wrap(err)
}

// pingUDP sends "ping" to the remote UDP server and wait for the
// "pong" response
func pingUDP(address string, duration time.Duration) error {
	conn, err := net.Dial("udp", address)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(duration))
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = conn.Write([]byte("ping"))
	if err != nil {
		return trace.Wrap(err)
	}
	buf := make([]byte, 4)
	_, err = conn.Read(buf)
	if err != nil {
		return trace.Wrap(err)
	}
	if strings.TrimSpace(string(buf)) != "pong" {
		return trace.BadParameter("unexpected response: %v", string(buf))
	}
	return nil
}
