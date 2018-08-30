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

package devicemapper

import (
	"fmt"
	"strconv"
	"text/scanner"

	"github.com/gravitational/trace"
)

// parser implements a parser for a single line of output from `lsblk -P --output=<list of columns>`
// The output is formatted as a series of key/value pairs:
//
// $ lsblk -P --output=NAME,MODE,OWNER
// NAME="xvda" MODE="brw-rw----" OWNER="root"
// NAME="xvda1" MODE="brw-rw----" OWNER="root"
// NAME="xvda2" MODE="brw-rw----" OWNER="root"
// NAME="xvdf" MODE="brw-rw----" OWNER="root"
//
// The parser returns a sequence of attr values that capture
// the attribute name/value details.
type parser struct {
	errors   []error
	scanner  scanner.Scanner
	position scanner.Position
	token    rune
	literal  string
}

func (r *parser) next() {
	r.token = r.scanner.Scan()
	r.position = r.scanner.Position
	r.literal = r.scanner.TokenText()
}

func (r *parser) parseAttribute() *attr {
	name := r.parseIdentifier()
	r.expect('=')
	value := r.parseLiteral()
	return &attr{
		name:  name,
		value: value,
	}
}

func (r *parser) parseIdentifier() string {
	name := r.literal
	r.expect(scanner.Ident)
	return name
}

func (r *parser) parseLiteral() string {
	value, _ := strconv.Unquote(r.literal)
	r.expect(scanner.String)
	return value
}

func (r *parser) expect(token rune) {
	if r.token != token {
		r.error(r.position, fmt.Sprintf("expected %v but got %v", scanner.TokenString(token),
			scanner.TokenString(r.token)))
	}
	r.next()
}

func (r *parser) error(pos scanner.Position, msg string) {
	r.errors = append(r.errors, trace.Errorf("%v: %v", pos, msg))
}

// attr defines an attribute - a name/value pair
type attr struct {
	name  string
	value string
}
