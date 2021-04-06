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

// Package reference provides a general type to represent any way of referencing images within the registry.
// Its main purpose is to abstract tags and digests (content-addressable hash).
//
// The source code is based on
// https://github.com/docker/distribution/blob/master/reference/reference.go
// with irrelevant parts removed.
//
// Grammar
//
// 	reference                       := name [ ":" tag ] [ "@" digest ]
//	name                            := [domain '/'] path-component ['/' path-component]*
//	domain                          := domain-component ['.' domain-component]* [':' port-number]
//	domain-component                := /([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])/
//	port-number                     := /[0-9]+/
//	path-component                  := alpha-numeric [separator alpha-numeric]*
// 	alpha-numeric                   := /[a-z0-9]+/
//	separator                       := /[_.]|__|[-]*/
//
//	tag                             := /[\w][\w.-]{0,127}/
//
//	digest                          := digest-algorithm ":" digest-hex
//	digest-algorithm                := digest-algorithm-component [ digest-algorithm-separator digest-algorithm-component ]*
//	digest-algorithm-separator      := /[+.-_]/
//	digest-algorithm-component      := /[A-Za-z][A-Za-z0-9]*/
//	digest-hex                      := /[0-9a-fA-F]{32,}/ ; At least 128 bit digest value
//
//	identifier                      := /[a-f0-9]{64}/
//	short-identifier                := /[a-f0-9]{6,64}/
package docker

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
)

// Parse parses s and returns a syntactically valid Reference.
// If an error was encountered it is returned, along with a nil Reference.
// NOTE: Parse will not handle short digests.
func Parse(s string) (Reference, error) {
	matches := ReferenceRegexp.FindStringSubmatch(s)
	if matches == nil {
		if s == "" {
			return nil, ErrNameEmpty
		}
		if ReferenceRegexp.FindStringSubmatch(strings.ToLower(s)) != nil {
			return nil, ErrNameContainsUppercase
		}
		return nil, ErrReferenceInvalidFormat
	}

	if len(matches[1]) > NameTotalLengthMax {
		return nil, ErrNameTooLong
	}

	var repo repository

	nameMatch := anchoredNameRegexp.FindStringSubmatch(matches[1])
	if len(nameMatch) == 3 {
		repo.domain = nameMatch[1]
		repo.path = nameMatch[2]
	} else {
		repo.domain = ""
		repo.path = matches[1]
	}

	ref := reference{
		namedRepository: repo,
		tag:             matches[2],
	}
	if matches[3] != "" {
		var err error
		ref.digest, err = digest.Parse(matches[3])
		if err != nil {
			return nil, err
		}
	}

	r := getBestReferenceType(ref)
	if r == nil {
		return nil, ErrNameEmpty
	}

	return r, nil
}

const (
	// NameTotalLengthMax is the maximum total number of characters in a repository name.
	NameTotalLengthMax = 255
)

var (
	// ErrReferenceInvalidFormat represents an error while trying to parse a string as a reference.
	ErrReferenceInvalidFormat = errors.New("invalid reference format")

	// ErrTagInvalidFormat represents an error while trying to parse a string as a tag.
	ErrTagInvalidFormat = errors.New("invalid tag format")

	// ErrDigestInvalidFormat represents an error while trying to parse a string as a tag.
	ErrDigestInvalidFormat = errors.New("invalid digest format")

	// ErrNameContainsUppercase is returned for invalid repository names that contain uppercase characters.
	ErrNameContainsUppercase = errors.New("repository name must be lowercase")

	// ErrNameEmpty is returned for empty, invalid repository names.
	ErrNameEmpty = errors.New("repository name must have at least one component")

	// ErrNameTooLong is returned when a repository name is longer than NameTotalLengthMax.
	ErrNameTooLong = fmt.Errorf("repository name must not be more than %v characters", NameTotalLengthMax)

	// ErrNameNotCanonical is returned when a name is not canonical.
	ErrNameNotCanonical = errors.New("repository name must be canonical")
)

// Reference is an opaque object reference identifier that may include
// modifiers such as a hostname, name, tag, and digest.
type Reference interface {
	// String returns the full reference
	String() string
}

// Named is an object with a full name
type Named interface {
	Reference
	Name() string
}

// Tagged is an object which has a tag
type Tagged interface {
	Reference
	Tag() string
}

// NamedTagged is an object including a name and tag.
type NamedTagged interface {
	Named
	Tag() string
}

// Digested is an object which has a digest
// in which it can be referenced by
type Digested interface {
	Reference
	Digest() digest.Digest
}

// Canonical reference is an object with a fully unique
// name including a name with domain and digest
type Canonical interface {
	Named
	Digest() digest.Digest
}

// namedRepository is a reference to a repository with a name.
// A namedRepository has both domain and path components.
type namedRepository interface {
	Named
	Domain() string
	Path() string
}

// Domain returns the domain part of the Named reference
func Domain(named Named) string {
	if r, ok := named.(namedRepository); ok {
		return r.Domain()
	}
	domain, _ := splitDomain(named.Name())
	return domain
}

// Path returns the name without the domain part of the Named reference
func Path(named Named) (name string) {
	if r, ok := named.(namedRepository); ok {
		return r.Path()
	}
	_, path := splitDomain(named.Name())
	return path
}

var (
	defaultDomain    = "docker.io"
	officialRepoName = "library"
)

// familiarizeName returns a shortened version of the name familiar
// to to the Docker UI. Familiar names have the default domain
// "docker.io" and "library/" repository prefix removed.
// For example, "docker.io/library/redis" will have the familiar
// name "redis" and "docker.io/dmcgowan/myapp" will be "dmcgowan/myapp".
// Returns a familiarized named only reference.
func familiarizeName(named namedRepository) repository {
	repo := repository{
		domain: named.Domain(),
		path:   named.Path(),
	}

	if repo.domain == defaultDomain {
		repo.domain = ""
		// Handle official repositories which have the pattern "library/<official repo name>"
		if split := strings.Split(repo.path, "/"); len(split) == 2 && split[0] == officialRepoName {
			repo.path = split[1]
		}
	}
	return repo
}

func (r reference) Familiar() Named {
	return reference{
		namedRepository: familiarizeName(r.namedRepository),
		tag:             r.tag,
		digest:          r.digest,
	}
}

func (r repository) Familiar() Named {
	return familiarizeName(r)
}

func (t taggedReference) Familiar() Named {
	return taggedReference{
		namedRepository: familiarizeName(t.namedRepository),
		tag:             t.tag,
	}
}

func (c canonicalReference) Familiar() Named {
	return canonicalReference{
		namedRepository: familiarizeName(c.namedRepository),
		digest:          c.digest,
	}
}

func splitDomain(name string) (string, string) {
	match := anchoredNameRegexp.FindStringSubmatch(name)
	if len(match) != 3 {
		return "", name
	}
	return match[1], match[2]
}

func getBestReferenceType(ref reference) Reference {
	if ref.Name() == "" {
		// Allow digest only references
		if ref.digest != "" {
			return digestReference(ref.digest)
		}
		return nil
	}
	if ref.tag == "" {
		if ref.digest != "" {
			return canonicalReference{
				namedRepository: ref.namedRepository,
				digest:          ref.digest,
			}
		}
		return ref.namedRepository
	}
	if ref.digest == "" {
		return taggedReference{
			namedRepository: ref.namedRepository,
			tag:             ref.tag,
		}
	}

	return ref
}

type reference struct {
	namedRepository
	tag    string
	digest digest.Digest
}

func (r reference) String() string {
	return r.Name() + ":" + r.tag + "@" + r.digest.String()
}

func (r reference) Tag() string {
	return r.tag
}

func (r reference) Digest() digest.Digest {
	return r.digest
}

type repository struct {
	domain string
	path   string
}

func (r repository) String() string {
	return r.Name()
}

func (r repository) Name() string {
	if r.domain == "" {
		return r.path
	}
	return r.domain + "/" + r.path
}

func (r repository) Domain() string {
	return r.domain
}

func (r repository) Path() string {
	return r.path
}

type digestReference digest.Digest

func (d digestReference) String() string {
	return digest.Digest(d).String()
}

func (d digestReference) Digest() digest.Digest {
	return digest.Digest(d)
}

type taggedReference struct {
	namedRepository
	tag string
}

func (t taggedReference) String() string {
	return t.Name() + ":" + t.tag
}

func (t taggedReference) Tag() string {
	return t.tag
}

type canonicalReference struct {
	namedRepository
	digest digest.Digest
}

func (c canonicalReference) String() string {
	return c.Name() + "@" + c.digest.String()
}

func (c canonicalReference) Digest() digest.Digest {
	return c.digest
}

var (
	// alphaNumericRegexp defines the alpha numeric atom, typically a
	// component of names. This only allows lower case characters and digits.
	alphaNumericRegexp = match(`[a-z0-9]+`)

	// separatorRegexp defines the separators allowed to be embedded in name
	// components. This allow one period, one or two underscore and multiple
	// dashes.
	separatorRegexp = match(`(?:[._]|__|[-]*)`)

	// nameComponentRegexp restricts registry path component names to start
	// with at least one letter or number, with following parts able to be
	// separated by one period, one or two underscore and multiple dashes.
	nameComponentRegexp = expression(
		alphaNumericRegexp,
		optional(repeated(separatorRegexp, alphaNumericRegexp)))

	// domainComponentRegexp restricts the registry domain component of a
	// repository name to start with a component as defined by DomainRegexp
	// and followed by an optional port.
	domainComponentRegexp = match(`(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])`)

	// DomainRegexp defines the structure of potential domain components
	// that may be part of image names. This is purposely a subset of what is
	// allowed by DNS to ensure backwards compatibility with Docker image
	// names.
	DomainRegexp = expression(
		domainComponentRegexp,
		optional(repeated(literal(`.`), domainComponentRegexp)),
		optional(literal(`:`), match(`[0-9]+`)))

	// TagRegexp matches valid tag names. From docker/docker:graph/tags.go.
	TagRegexp = match(`[\w][\w.-]{0,127}`)

	// DigestRegexp matches valid digests.
	DigestRegexp = match(`[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,}`)

	// NameRegexp is the format for the name component of references. The
	// regexp has capturing groups for the domain and name part omitting
	// the separating forward slash from either.
	NameRegexp = expression(
		optional(DomainRegexp, literal(`/`)),
		nameComponentRegexp,
		optional(repeated(literal(`/`), nameComponentRegexp)))

	// anchoredNameRegexp is used to parse a name value, capturing the
	// domain and trailing components.
	anchoredNameRegexp = anchored(
		optional(capture(DomainRegexp), literal(`/`)),
		capture(nameComponentRegexp,
			optional(repeated(literal(`/`), nameComponentRegexp))))

	// ReferenceRegexp is the full supported format of a reference. The regexp
	// is anchored and has capturing groups for name, tag, and digest
	// components.
	ReferenceRegexp = anchored(capture(NameRegexp),
		optional(literal(":"), capture(TagRegexp)),
		optional(literal("@"), capture(DigestRegexp)))

	// IdentifierRegexp is the format for string identifier used as a
	// content addressable identifier using sha256. These identifiers
	// are like digests without the algorithm, since sha256 is used.
	IdentifierRegexp = match(`([a-f0-9]{64})`)

	// ShortIdentifierRegexp is the format used to represent a prefix
	// of an identifier. A prefix may be used to match a sha256 identifier
	// within a list of trusted identifiers.
	ShortIdentifierRegexp = match(`([a-f0-9]{6,64})`)
)

// match compiles the string to a regular expression.
var match = regexp.MustCompile

// literal compiles s into a literal regular expression, escaping any regexp
// reserved characters.
func literal(s string) *regexp.Regexp {
	re := match(regexp.QuoteMeta(s))

	if _, complete := re.LiteralPrefix(); !complete {
		panic("must be a literal")
	}

	return re
}

// expression defines a full expression, where each regular expression must
// follow the previous.
func expression(res ...*regexp.Regexp) *regexp.Regexp {
	var s string
	for _, re := range res {
		s += re.String()
	}

	return match(s)
}

// optional wraps the expression in a non-capturing group and makes the
// production optional.
func optional(res ...*regexp.Regexp) *regexp.Regexp {
	return match(group(expression(res...)).String() + `?`)
}

// repeated wraps the regexp in a non-capturing group to get one or more
// matches.
func repeated(res ...*regexp.Regexp) *regexp.Regexp {
	return match(group(expression(res...)).String() + `+`)
}

// group wraps the regexp in a non-capturing group.
func group(res ...*regexp.Regexp) *regexp.Regexp {
	return match(`(?:` + expression(res...).String() + `)`)
}

// capture wraps the expression in a capturing group.
func capture(res ...*regexp.Regexp) *regexp.Regexp {
	return match(`(` + expression(res...).String() + `)`)
}

// anchored anchors the regular expression by adding start and end delimiters.
func anchored(res ...*regexp.Regexp) *regexp.Regexp {
	return match(`^` + expression(res...).String() + `$`)
}

func parseNamed(name string) (Named, error) {
	ref, err := Parse(name)
	if err != nil {
		return nil, trace.Wrap(err, "invalid named reference %q", name)
	}

	if named, ok := ref.(Named); ok {
		return named, nil
	}

	return nil, trace.BadParameter("failed to parse named reference %q", name)
}
