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
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"text/scanner"

	"github.com/docker/docker/pkg/idtools"
	"github.com/gravitational/trace"
)

// exclusion return true if the specified pattern is an exclusion
func exclusion(pattern string) bool {
	return pattern[0] == '!'
}

// empty return true if the specified pattern is empty
func empty(pattern string) bool {
	return pattern == ""
}

// splitPathDirEntry splits the given path between its
// parent directory and its basename in that directory.
func splitPathDirEntry(localizedPath string) (dir, base string) {
	normalizedPath := filepath.ToSlash(localizedPath)
	vol := filepath.VolumeName(normalizedPath)
	normalizedPath = normalizedPath[len(vol):]

	if normalizedPath == "/" {
		// Specifies the root path.
		return filepath.FromSlash(vol + normalizedPath), "."
	}

	trimmedPath := vol + strings.TrimRight(normalizedPath, "/")

	dir = filepath.FromSlash(path.Dir(trimmedPath))
	base = filepath.FromSlash(path.Base(trimmedPath))

	return dir, base
}

// cleanPatterns takes a slice of patterns returns a new
// slice of patterns cleaned with filepath.Clean, stripped
// of any empty patterns and lets the caller know whether the
// slice contains any exception patterns (prefixed with !).
func cleanPatterns(patterns []string) ([]string, [][]string, bool, error) {
	// Loop over exclusion patterns and:
	// 1. Clean them up.
	// 2. Indicate whether we are dealing with any exception rules.
	// 3. Error if we see a single exclusion marker on it's own (!).
	cleanedPatterns := []string{}
	patternDirs := [][]string{}
	exceptions := false
	for _, pattern := range patterns {
		// Eliminate leading and trailing whitespace.
		pattern = strings.TrimSpace(pattern)
		if empty(pattern) {
			continue
		}
		if exclusion(pattern) {
			if len(pattern) == 1 {
				return nil, nil, false, trace.BadParameter("illegal exclusion pattern: %v", pattern)
			}
			exceptions = true
		}
		pattern = filepath.Clean(pattern)
		cleanedPatterns = append(cleanedPatterns, pattern)
		if exclusion(pattern) {
			pattern = pattern[1:]
		}
		patternDirs = append(patternDirs, strings.Split(pattern, "/"))
	}

	return cleanedPatterns, patternDirs, exceptions, nil
}

// OptimizedMatches is basically the same as fileutils.Matches() but optimized for archive.go.
// It will assume that the inputs have been preprocessed and therefore the function
// doen't need to do as much error checking and clean-up. This was done to avoid
// repeating these steps on each file being checked during the archive process.
// The more generic fileutils.Matches() can't make these assumptions.
func optimizedMatches(file string, patterns []string, patDirs [][]string) (bool, error) {
	matched := false
	parentPath := filepath.Dir(file)
	parentPathDirs := strings.Split(parentPath, "/")

	for i, pattern := range patterns {
		negative := false

		if exclusion(pattern) {
			negative = true
			pattern = pattern[1:]
		}

		match, err := PathMatch(PathPattern(pattern), file)
		if err != nil {
			return false, err
		}

		if !match && parentPath != "." {
			// Check to see if the pattern matches one of our parent dirs.
			if len(patDirs[i]) <= len(parentPathDirs) {
				match, _ = filepath.Match(strings.Join(patDirs[i], "/"),
					strings.Join(parentPathDirs[:len(patDirs[i])], "/"))
			}
		}

		if match {
			matched = !negative
		}
	}

	return matched, nil
}

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
