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

package archive

import (
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"text/scanner"

	"github.com/docker/docker/pkg/idtools"
	"github.com/gravitational/trace"
)

// PathPattern defines a type for a file path pattern
type PathPattern string

// PathMatch matches path against the specified path pattern.
// The pattern can use double-asterisks (`**`) to denote arbitrary intermediate directories
// in the path.
// Returns True upon a successful match and error if the pattern is invalid.
// Based on docker/docker/pkg/fileutils/fileutils#regexpMatch
func PathMatch(pattern PathPattern, path string) (bool, error) {
	expr := "^"

	if _, err := filepath.Match(string(pattern), path); err != nil {
		return false, trace.Wrap(err)
	}

	var scan scanner.Scanner
	scan.Init(strings.NewReader(string(pattern)))

	sep := string(os.PathSeparator)
	escapedSep := sep
	if sep == `\` {
		escapedSep += `\`
	}

	for scan.Peek() != scanner.EOF {
		ch := scan.Next()

		switch ch {
		case '*':
			if scan.Peek() == '*' {
				// is some flavor of "**"
				scan.Next()

				if scan.Peek() == scanner.EOF {
					// "**EOF" - accept all
					expr += ".*"
				} else {
					// "**"
					expr += "((.*" + escapedSep + ")|([^" + escapedSep + "]*))"
				}

				// treat **/ as ** so eat the "/"
				if string(scan.Peek()) == sep {
					scan.Next()
				}
			} else {
				// is "*" so map it to anything but path separator
				expr += "[^" + escapedSep + "]*"
			}
		case '?':
			// "?" is any char except a path separator
			expr += "[^" + escapedSep + "]"
		case '.', '$':
			// escape some regexp special chars that have no meaning
			// in filename match
			expr += `\` + string(ch)
		case '\\':
			// escape next char. Note that a trailing \ in the pattern
			// will be left alone but needs to be escaped
			if scan.Peek() != scanner.EOF {
				expr += `\` + string(scan.Next())
			} else {
				expr += `\`
			}
		default:
			expr += string(ch)
		}
	}

	expr += "$"
	matches, err := regexp.MatchString(expr, path)

	return matches, trace.Wrap(err)
}

// GetChownOptionsForDir returns the ownership options for the specified directory dir.
// It will use the same options if directory already exists, and will fall back to current
// user otherwise
func GetChownOptionsForDir(dir string) (*idtools.Identity, error) {
	var uid, gid int
	// preserve owner/group when unpacking, otherwise use current process user
	fi, err := os.Stat(dir)
	if err == nil && fi.Sys() != nil {
		switch stat := fi.Sys().(type) {
		case *syscall.Stat_t:
			uid = int(stat.Uid)
			gid = int(stat.Gid)
			return &idtools.Identity{
				UID: uid,
				GID: gid,
			}, nil
		}
	}
	user, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query current user")
	}
	uid, err = strconv.Atoi(user.Uid)
	if err != nil {
		return nil, trace.BadParameter("UID is not a number: %q", user.Uid)
	}
	gid, err = strconv.Atoi(user.Gid)
	if err != nil {
		return nil, trace.BadParameter("GID is not a number: %q", user.Gid)
	}
	return &idtools.Identity{
		UID: uid,
		GID: gid,
	}, nil
}
