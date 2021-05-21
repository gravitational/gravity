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

package systeminfo

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"
)

// ResolvFromFile reads the given resolv.conf file
func ResolvFromFile(filename string) (*storage.ResolvConf, error) {
	path, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer f.Close()
	cfg, err := ResolvFromReader(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cfg, nil
}

// ResolvFromReader reads the given resolv.conf as an io.Reader
func ResolvFromReader(rdr io.Reader) (*storage.ResolvConf, error) {
	// initialize with some defaults
	resolv := &storage.ResolvConf{
		Ndots:    1,
		Timeout:  5,
		Attempts: 2,
	}

	scanner := bufio.NewScanner(rdr)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, trace.Wrap(err)
		}

		line := scanner.Text()

		// strip comments
		line = strings.Split(line, ";")[0]
		line = strings.Split(line, "#")[0]

		f := strings.Fields(line)
		if len(f) < 1 {
			continue
		}

		switch f[0] {
		case "nameserver":
			if len(f) > 1 {
				resolv.Servers = append(resolv.Servers, f[1])
			}

		case "domain":
			if len(f) > 1 {
				resolv.Domain = f[1]
				resolv.Search = []string{f[1]}
			}

		case "search":
			resolv.Search = make([]string, len(f)-1)
			for i := 0; i < len(resolv.Search); i++ {
				resolv.Search[i] = f[i+1]
			}

		case "options": // magic options
			for _, s := range f[1:] {
				switch {
				case strings.HasPrefix(s, "ndots:"):
					n, _ := strconv.Atoi(s[6:])
					if n < 1 {
						n = 1
					}
					resolv.Ndots = n
				case strings.HasPrefix(s, "timeout:"):
					n, _ := strconv.Atoi(s[8:])
					if n < 1 {
						n = 1
					}
					resolv.Timeout = n
				case strings.HasPrefix(s, "attempts:"):
					n, _ := strconv.Atoi(s[9:])
					if n < 1 {
						n = 1
					}
					resolv.Attempts = n
				case s == "rotate":
					resolv.Rotate = true
				default:
					resolv.UnknownOpt = true
				}
			}

		case "lookup":
			// OpenBSD option:
			// http://www.openbsd.org/cgi-bin/man.cgi/OpenBSD-current/man5/resolv.conf.5
			// "the legal space-separated values are: bind, file, yp"
			resolv.Lookup = f[1:]

		default:
			resolv.UnknownOpt = true
		}
	}

	return resolv, nil
}
