/*
Copyright 2017 Gravitational, Inc.

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

package monitoring

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"github.com/prometheus/procfs"
	log "github.com/sirupsen/logrus"
)

type portCollectorFunc func() ([]process, error)

func realGetPorts() ([]process, error) {
	return getPorts(fetchAllProcs, getTCPSockets, getTCP6Sockets, getUDPSockets, getUDP6Sockets)
}

func getPorts(getProcs procGetterFunc, getSockets ...socketGetterFunc) ([]process, error) {
	procs, err := getProcs()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query running processes")
	}
	inodes := mapAllProcs(procs)
	collector := portCollector{inodes}
	processes, err := collector.sockets(getSockets...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query socket connections")
	}
	return processes, nil
}

func (r portCollector) sockets(fetchSockets ...socketGetterFunc) (ret []process, err error) {
	var sockets []socket
	for _, fn := range fetchSockets {
		batch, err := fn()
		if err != nil && !trace.IsNotFound(err) {
			// Ignore if not enabled
			return nil, trace.Wrap(err, "failed to query sockets")
		}
		sockets = append(sockets, batch...)
	}

	for _, socket := range sockets {
		proc, err := r.findProcessByInode(socket.inode())
		if err != nil {
			log.Warn(err.Error())
		}
		ret = append(ret, process{
			name:   proc.name,
			pid:    proc.pid,
			socket: socket,
		})
	}
	return ret, nil
}

func (r portCollector) findProcessByInode(inode string) (proc process, err error) {
	proc = process{pid: pidUnknown, name: valueUnknown}
	if pid, exists := r.inodes[inode]; exists {
		proc.pid = pid
	}

	comm := valueUnknown
	if proc.pid != pidUnknown {
		p, err := procfs.NewProc(int(proc.pid))
		if err != nil {
			return proc, trace.Wrap(err, "failed to query process metadata for pid %v", proc.pid)
		}
		comm, err = p.Comm()
		if err != nil {
			return proc, trace.Wrap(err, "failed to query process name for pid %v", proc.pid)
		}
		proc.name = comm
		return proc, nil
	}
	return proc, trace.NotFound("no process found for inode %v", inode)
}

type portCollector struct {
	inodes map[string]pid
}

type procGetterFunc func() (procfs.Procs, error)

func fetchAllProcs() (procfs.Procs, error) {
	procs, err := procfs.AllProcs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return procs, nil
}

type process struct {
	name string
	pid  pid
	socket
}

func (r tcpSocket) localAddr() addr {
	return addr{r.LocalAddress.IP, r.LocalAddress.Port}
}

func (r tcpSocket) inode() string {
	return strconv.FormatUint(uint64(r.Inode), 10)
}

func (r tcpSocket) state() socketState {
	return r.State
}

func (r tcpSocket) proto() string {
	return "tcp"
}

func (r udpSocket) localAddr() addr {
	return addr{r.LocalAddress.IP, r.LocalAddress.Port}
}

func (r udpSocket) inode() string {
	return strconv.FormatUint(uint64(r.Inode), 10)
}

func (r udpSocket) state() socketState {
	return r.State
}

func (r udpSocket) proto() string {
	return "udp"
}

// getTCPSockets interprets the file specified with fpath formatted as /proc/net/tcp{,6}
// and returns a list of TCPSockets
func getTCPSocketsFromPath(fpath string) (ret []socket, err error) {
	err = parseSocketFile(fpath, func(line string) error {
		socket, err := newTCPSocketFromLine(line)
		if err != nil {
			return trace.Wrap(err)
		}
		ret = append(ret, *socket)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ret, nil
}

// getUDPSockets interprets the file specified with fpath formatted as /proc/net/udp{,6}
// and returns a list of UDPPSockets
func getUDPSocketsFromPath(fpath string) (ret []socket, err error) {
	err = parseSocketFile(fpath, func(line string) error {
		socket, err := newUDPSocketFromLine(line)
		if err != nil {
			return trace.Wrap(err)
		}
		ret = append(ret, *socket)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ret, nil
}

type socketGetterFunc func() ([]socket, error)

func getTCPSockets() ([]socket, error) {
	return getTCPSocketsFromPath(procNetTCP)
}
func getTCP6Sockets() ([]socket, error) {
	return getTCPSocketsFromPath(procNetTCP6)
}

func getUDPSockets() ([]socket, error) {
	return getUDPSocketsFromPath(procNetUDP)
}
func getUDP6Sockets() ([]socket, error) {
	return getUDPSocketsFromPath(procNetUDP6)
}

func getUnixSockets() ([]unixSocket, error) {
	return getUnixSocketsFromPath(procNetUnix)
}

// getUnixSockets interprets the file specified with fpath formatted as /proc/net/unix
// and returns a list of UnixPSockets
func getUnixSocketsFromPath(fpath string) (ret []unixSocket, err error) {
	err = parseSocketFile(fpath, func(line string) error {
		socket, err := newUnixSocketFromLine(line)
		if err != nil {
			return trace.Wrap(err)
		}
		ret = append(ret, *socket)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ret, nil
}

func parseSocketFile(fpath string, parse socketparser) error {
	fp, err := os.Open(fpath)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer fp.Close()
	lineScanner := bufio.NewScanner(fp)
	lineScanner.Scan() // Drop header line
	for lineScanner.Scan() {
		err := parse(lineScanner.Text())
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if err := lineScanner.Err(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type socketparser func(string) error

// newTCPSocketFromLine parses a TCPSocket from a line in /proc/net/tcp{,6}
func newTCPSocketFromLine(line string) (*tcpSocket, error) {
	// sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
	//  0: 00000000:0035 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 18616 1 ffff91e759d47080 100 0 0 10 0
	// reference: https://github.com/ecki/net-tools/blob/master/netstat.c#L1070
	var (
		sl         int
		localip    []byte
		remoteip   []byte
		tr         int
		tmwhen     int
		retransmit int
		timeout    int
		tails      string
		socket     tcpSocket
	)
	_, err := fmt.Sscanf(line, "%d: %32X:%4X %32X:%4X %2X %8X:%8X %2X:%8X %8X %d %d %d %1s",
		&sl, &localip, &socket.LocalAddress.Port, &remoteip, &socket.RemoteAddress.Port,
		&socket.State, &socket.TXQueue, &socket.RXQueue, &tr, &tmwhen, &retransmit,
		&socket.UID, &timeout, &socket.Inode, &tails)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	socket.LocalAddress.IP = hexToIP(localip)
	socket.RemoteAddress.IP = hexToIP(remoteip)
	return &socket, nil
}

// newUDPSocketFromLine parses a UDPSocket from a line in /proc/net/udp{,6}
func newUDPSocketFromLine(line string) (*udpSocket, error) {
	//    sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops
	//  2511: 00000000:14E9 00000000:0000 07 00000000:00000000 00:00000000 00000000  1000        0 1662497 2 ffff91e6a9fcbc00 0
	var (
		sl         int
		localip    []byte
		remoteip   []byte
		tr         int
		tmwhen     int
		retransmit int
		timeout    int
		pointer    []byte
		socket     udpSocket
	)
	_, err := fmt.Sscanf(line, "%d: %32X:%4X %32X:%4X %2X %8X:%8X %2X:%8X %8X %d %d %d %d %128X %d",
		&sl, &localip, &socket.LocalAddress.Port, &remoteip, &socket.RemoteAddress.Port,
		&socket.State, &socket.TXQueue, &socket.RXQueue, &tr, &tmwhen, &retransmit,
		&socket.UID, &timeout, &socket.Inode, &socket.RefCount, &pointer,
		&socket.Drops)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	socket.LocalAddress.IP = hexToIP(localip)
	socket.RemoteAddress.IP = hexToIP(remoteip)
	return &socket, nil
}

// newUnixSocketFromLine parses a UnixSocket from a line in /proc/net/unix
func newUnixSocketFromLine(line string) (*unixSocket, error) {
	// Num               RefCount Protocol Flags    Type St Inode Path
	// ffff91e759dfb800: 00000002 00000000 00010000 0001 01 16163 /tmp/sddm-auth3949710e-7c3f-4aa2-b5fc-25cc34a7f31e
	var pointer []byte
	var socket unixSocket
	n, err := fmt.Sscanf(line, "%128X: %8X %8X %8X %4X %2X %d %32000s",
		&pointer, &socket.RefCount, &socket.Protocol, &socket.Flags,
		&socket.Type, &socket.State, &socket.Inode, &socket.Path)
	if err != nil && n < 7 {
		return nil, trace.Wrap(err)
	}
	return &socket, nil
}

// hexToIP converts byte slice to net.IP
func hexToIP(in []byte) net.IP {
	ip := make([]byte, len(in))
	for i, v := range in {
		ip[len(ip)-i-1] = v
	}
	return ip
}

// mapAllProcs builds a mapping of inode values to running processes.
// It will skip processes it does not have access to.
func mapAllProcs(procs procfs.Procs) (inodes map[string]pid) {
	inodes = make(map[string]pid, len(procs))
	for _, proc := range procs {
		err := mapPidToInode(pid(proc.PID), inodes)
		if err != nil {
			log.Warnf("failed to associate sockets with process %v: %v", proc.PID, err)
			continue
		}
	}
	return inodes
}

// mapPidToInode associates the process given with pid with the sockets it has opened
func mapPidToInode(pid pid, inodes map[string]pid) error {
	dir := fdDir(pid)
	f, err := os.Open(dir)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()

	files, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, f := range files {
		inodePath := filepath.Join(dir, f)
		inode, err := os.Readlink(inodePath)
		if err != nil {
			log.Debugf("failed to readlink(%q): %v", inodePath, err)
			continue
		}
		if !strings.HasPrefix(inode, socketPrefix) {
			continue
		}
		// Extract inode value from 'socket:[inode]'
		inode = inode[len(socketPrefix) : len(inode)-1]
		if _, exists := inodes[inode]; !exists {
			inodes[inode] = pid
		}
	}
	return nil
}

func fdDir(pid pid) string {
	return filepath.Join("/proc", pid.String(), "fd")
}

func (r pid) String() string {
	return strconv.FormatInt(int64(r), 10)
}

type pid int

const socketPrefix = "socket:["

// valueUnknown specifies a value placeholder when the actual data is unavailable
const valueUnknown = "<unknown>"

// pidUnknown identifies a process we failed to match to an id
const pidUnknown = -1

type socket interface {
	localAddr() addr
	inode() string
	state() socketState
	proto() string
}

type addr struct {
	ip   net.IP
	port int
}

// String returns the address' string representation
func (a addr) String() string {
	return fmt.Sprintf("%s:%v", a.ip, a.port)
}

// formatSocket formats the provided socket to a string
func formatSocket(s socket) string {
	return fmt.Sprintf("socket(proto=%v, addr=%s, state=%s, inode=%v)",
		s.proto(), s.localAddr(), s.state(), s.inode())
}

// procNet* are standard paths to Linux procfs information on sockets
const (
	procNetTCP  = "/proc/net/tcp"
	procNetTCP6 = "/proc/net/tcp6"
	procNetUDP  = "/proc/net/udp"
	procNetUDP6 = "/proc/net/udp6"
	procNetUnix = "/proc/net/unix"
)

func (r socketState) String() string {
	switch r {
	case Established:
		return "established"
	case SynSent:
		return "syn-sent"
	case SynRecv:
		return "syn-recv"
	case FinWait1:
		return "fin-wait-1"
	case FinWait2:
		return "fin-wait-2"
	case TimeWait:
		return "time-wait"
	case Close:
		return "close"
	case CloseWait:
		return "close-wait"
	case LastAck:
		return "last-ack"
	case Listen:
		return "listen"
	case Closing:
		return "closing"
	}
	return strconv.FormatInt(int64(r), 10)
}

// SocketState stores Linux socket state
type socketState uint8

// Possible Linux socket states
const (
	Established socketState = 0x01
	SynSent                 = 0x02
	SynRecv                 = 0x03
	FinWait1                = 0x04
	FinWait2                = 0x05
	TimeWait                = 0x06
	Close                   = 0x07
	CloseWait               = 0x08
	LastAck                 = 0x09
	Listen                  = 0x0A
	Closing                 = 0x0B
)

// tcpSocket contains details on a TCP socket
type tcpSocket struct {
	LocalAddress  net.TCPAddr
	RemoteAddress net.TCPAddr
	State         socketState
	TXQueue       uint
	RXQueue       uint
	TimerState    uint
	TimeToTimeout uint
	Retransmit    uint
	UID           uint
	Inode         uint
}

// udpSocket contains details on a UDP socket
type udpSocket struct {
	LocalAddress  net.UDPAddr
	RemoteAddress net.UDPAddr
	State         socketState
	TXQueue       uint
	RXQueue       uint
	UID           uint
	Inode         uint
	RefCount      uint
	Drops         uint
}

// unixSocket contains details on a unix socket
type unixSocket struct {
	RefCount uint
	Protocol uint
	Flags    uint
	Type     uint
	State    uint
	Inode    uint
	Path     string
}
