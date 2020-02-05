/*
Copyright 2020 Gravitational, Inc.

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

package schema

import (
	"bufio"
	"io"
	"io/ioutil"
	"strings"

	"github.com/google/shlex"
	"github.com/gravitational/trace"
	"gopkg.in/alecthomas/kingpin.v2"
)

// ParseFromString parses port command from the specified string
func (r *PortCommand) ParseFromString(input string) (err error) {
	l := shlex.NewLexer(strings.NewReader(input))
	var args []string
L:
	for err == nil {
		var token string
		token, err = l.Next()
		if err != nil {
			if err == io.EOF {
				break L
			}
			return trace.Wrap(err)
		}
		args = append(args, token)
	}
	return r.Parse(args)
}

// Parse parses port command from the specified arguments
func (r *PortCommand) Parse(args []string) error {
	app := kingpin.New("semanage", "Policy management tool")
	app.Writer(ioutil.Discard)
	app.Terminate(func(int) {})
	cmd := app.Command("port", "Port mapping tool")
	cmd.Flag("type", "SELinux type").Short('t').StringVar(&r.Type)
	_ = cmd.Flag("add", "Add a new port record").Short('a').Bool()
	_ = cmd.Flag("delete", "Delete port record").Short('d').Bool()
	cmd.Flag("range", "MLS/MCS security range. Defaults to s0").Short('r').Default("s0").StringVar(&r.SecurityRange)
	cmd.Flag("proto", "Protocol or internet protocol version").Short('p').StringVar(&r.Protocol)
	cmd.Arg("port-range", "Port value or port range as 'start-end'").StringVar(&r.Range)
	_, err := app.Parse(args)
	return trace.Wrap(err)
}

// GetLocalPortChangesFromReader interprets the specified reader contents
// as a sequence of 'semanage port' commands
func GetLocalPortChangesFromReader(r io.Reader) ([]PortCommand, error) {
	s := bufio.NewScanner(r)
	s.Split(bufio.ScanLines)
	var ports []PortCommand
	for s.Scan() {
		cmd := strings.TrimSpace(s.Text())
		if cmd == "" {
			continue
		}
		var c PortCommand
		if err := c.ParseFromString(cmd); err != nil {
			return nil, trace.Wrap(err)
		}
		ports = append(ports, c)
	}
	if err := s.Err(); err != nil {
		return nil, trace.Wrap(err)
	}
	return ports, nil
}

// PortCommand provides syntax support for the 'semanage port' command
type PortCommand struct {
	// Type specifies the SELinux type for the port object
	Type string
	// MLS/MCS Security range (MLS/MCS systems only).
	// SELinux range for SELinux user; defaults to s0
	SecurityRange string
	// Protocol for the specified port (tcp|udp) or internet protocol
	// version for the specified node (ipv4|ipv6)
	Protocol string
	// Range specifies the port range value.
	// Can specify either a single value like '7000' or a range like '7000-7002'
	Range string
}
