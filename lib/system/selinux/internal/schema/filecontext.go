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
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/scanner"
	"unicode"

	"github.com/gravitational/trace"
)

// ParseFcontextFile parses a filecontext file given with r.
// The parser is simple and line-driven hence it does not support
// complex constructs like 'ifdef'.
func ParseFcontextFile(r io.Reader) (result []FcontextFileItem, err error) {
	s := bufio.NewScanner(r)
	s.Split(bufio.ScanLines)
	for s.Scan() {
		t := bytes.TrimSpace(s.Bytes())
		if len(t) == 0 || bytes.HasPrefix(t, []byte("#")) {
			continue
		}
		item, err := parseFcontextFileItem(string(t))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, *item)
	}
	if s.Err() != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

// AsAddCommand formats this item as a 'semanage fcontext' command
// to add a new local rule
func (r FcontextFileItem) AsAddCommand() string {
	if r.Label == nil {
		return ""
	}
	return fmt.Sprintf("fcontext --add --ftype %v --type %v --range '%v' '%v'",
		r.FileType.AsParameter(),
		r.Label.Type,
		r.Label.SecurityRange,
		r.Path,
	)
}

// AsRemoveCommand formats this item as a 'semanage fcontext' command
// to remove an existing local rule
func (r FcontextFileItem) AsRemoveCommand() string {
	return fmt.Sprintf("fcontext --delete --ftype %v '%v'",
		r.FileType.AsParameter(),
		r.Path,
	)
}

// FcontextFileItem describes a single item from the filecontext file.
// References: https://selinuxproject.org/page/NB_RefPolicy and
// https://www.systutorials.com/docs/linux/man/5-file_contexts/
type FcontextFileItem struct {
	// Path specifies the path this entry is configuring
	Path string
	// FileType specifies the file type this entry
	// targets: any, regular files, sockets, symlinks, etc.
	FileType FileType
	// Label specifies the SELinux label of this entry
	Label *Label
}

// String returns a text representation of this SELinux context
func (r Label) String() string {
	return fmt.Sprintf("%v:%v:%v:%v",
		r.User, r.Role, r.Type, r.SecurityRange)
}

// Label describes an SELinux label
type Label struct {
	// User specifies the SELinux user
	User string
	// Role specifies the SELinux role
	Role string
	// Type specifies the SELinux resource type
	Type string
	// SecurityRange specifies the MCS/MLS security range
	SecurityRange string
}

// AsParameter converts this file type to a type value compatible
// with 'semanage fcontext' command.
// See: semanage-fcontext(8)
func (r FileType) AsParameter() string {
	switch r {
	case RegularFile:
		return "f"
	case BlockDevice:
		return "b"
	case CharDevice:
		return "c"
	case NamedPipe:
		return "p"
	case Socket:
		return "s"
	case Symlink:
		return "l"
	case Directory:
		return "d"
	default:
		return "a"
	}
}

// String returns text representation of this file type
func (r FileType) String() string {
	switch r {
	case RegularFile:
		return "--"
	case BlockDevice:
		return "-b"
	case CharDevice:
		return "-c"
	case NamedPipe:
		return "-p"
	case Socket:
		return "-s"
	case Symlink:
		return "-l"
	case Directory:
		return "-d"
	default:
		// AllFiles
		return "  "
	}
}

// FileType describes the type of file specified by a single filecontext item
type FileType uint8

const (
	// AllFiles represents any file type
	AllFiles FileType = iota
	// RegularFile represents a regular file
	RegularFile
	// BlockDevice represents a block device file
	BlockDevice
	// CharDevice represents a character device file
	CharDevice
	// NamedPipe represents a named pipe
	NamedPipe
	// Socket represents a socket file
	Socket
	// Symlink represents a symbolic link
	Symlink
	// Directory represents a directory
	Directory
)

// Reference: https://github.com/SELinuxProject/selinux/blob/master/libsemanage/src/fcontexts_file.c#L81
func parseFcontextFileItem(s string) (item *FcontextFileItem, err error) {
	s = strings.TrimSpace(s)
	index := strings.IndexFunc(s, isWhitespace)
	if index == -1 {
		return nil, trace.BadParameter("invalid filecontext: expected '<path> [<file type>] <context>' but got %q",
			s)
	}
	var fileType, context string
	// First segment is the path
	path := s[:index]
	s = strings.TrimSpace(s[index:])
	if hasFileTypePrefix(s) {
		index = strings.IndexFunc(s, isWhitespace)
		if index == -1 {
			return nil, trace.BadParameter("invalid filecontext: expected '<file type> <context>' but got %q",
				s)
		}
		// Have file type segment
		fileType = s[:index]
		context = strings.TrimSpace(s[index:])
	} else {
		// Have no file type segment
		context = s
	}
	label, err := parseContext(context)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ftype, err := parseFileType(fileType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &FcontextFileItem{
		Path:     path,
		FileType: *ftype,
		Label:    label,
	}, nil
}

func hasFileTypePrefix(s string) bool {
	if len(s) < 2 {
		return false
	}
	_, err := parseFileType(s[:2])
	return err == nil
}

func parseFileType(s string) (*FileType, error) {
	r := AllFiles
	if strings.TrimSpace(s) == "" {
		return &r, nil
	}
	switch s {
	case "--":
		r = RegularFile
	case "-b":
		r = BlockDevice
	case "-c":
		r = CharDevice
	case "-p":
		r = NamedPipe
	case "-s":
		r = Socket
	case "-l":
		r = Symlink
	case "-d":
		r = Directory
	default:
		return nil, trace.BadParameter("invalid file type %q", s)
	}
	return &r, nil
}

func parseContext(s string) (*Label, error) {
	if s == "<<none>>" {
		// Relabelling application should not relabel this location
		return nil, nil
	}
	p := newContextParser(s)
	label, err := p.parse()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return label, nil
}

func newContextParser(s string) *contextParser {
	return &contextParser{
		s: s,
		t: newTokenizer(strings.NewReader(s)),
	}
}

func (p *contextParser) parse() (*Label, error) {
	p.ident("gen_context")
	p.skip("(")
	p.label()
	p.skip(",")
	p.securityRange()
	p.skip(")")
	return p.state, p.err
}

func (p *contextParser) ident(tok string) {
	typ := p.next()
	if p.t.Text() != tok || typ != scanner.Ident {
		p.error("expected identifier %q but got %q in %v at %v", tok, p.t.Text(), p.s, p.t.Position())
		return
	}
}

func (p *contextParser) skip(tok string) {
	p.next()
	if p.t.Text() != tok {
		p.error("expected %q but got %q in %v at %v", tok, p.t.Text(), p.s, p.t.Position())
		return
	}
}

func (p *contextParser) label() {
	p.next()
	pieces := strings.SplitN(p.t.Text(), ":", 3)
	if len(pieces) != 3 {
		p.error("invalid security context: expected '<user>:<role>:<type>' but got %q", p.t.Text())
		return
	}
	p.state = &Label{
		User: pieces[0],
		Role: pieces[1],
		Type: pieces[2],
	}
}

func (p *contextParser) securityRange() {
	p.next()
	p.state.SecurityRange = p.t.Text()
}

func (p *contextParser) next() rune {
	if p.err != nil {
		return scanner.EOF
	}
	ch := p.t.Next()
	if ch == scanner.EOF {
		p.err = io.EOF
	}
	return ch
}

func (p *contextParser) error(format string, args ...interface{}) {
	if p.err == nil {
		p.err = trace.BadParameter(format, args...)
	}
}

type contextParser struct {
	t     *tokenizer
	s     string
	state *Label
	err   error
}

func newTokenizer(r io.Reader) *tokenizer {
	var s scanner.Scanner
	s.Init(r)
	s.Filename = "input"
	s.IsIdentRune = isIdentRune
	t := &tokenizer{s: &s}
	return t
}

func isWhitespace(ch rune) bool {
	return unicode.IsSpace(ch)
}

func (r tokenizer) Next() rune {
	return r.s.Scan()
}

func (r tokenizer) Position() scanner.Position {
	return r.s.Pos()
}

func (r tokenizer) Text() string {
	return r.s.TokenText()
}

func isIdentRune(ch rune, index int) bool {
	switch ch {
	case ':':
		return index > 0
	case '_':
		return true
	}
	return unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

type tokenizer struct {
	s *scanner.Scanner
}
