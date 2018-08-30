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

package gce

import (
	"regexp"

	"github.com/gravitational/trace"
)

// ValidateTag validates the tag value for conformance
func ValidateTag(tag string) error {
	if len(tag) == 0 {
		return trace.BadParameter("tag value cannot be empty")
	}
	var errors []error
	if len(tag) > maxTagLength {
		errors = append(errors,
			trace.BadParameter("tag value cannot be longer than %v characters", maxTagLength))
	}
	if !tagRegex.MatchString(tag) && !tagSingleCharacterRegex.MatchString(tag) {
		errors = append(errors,
			trace.BadParameter("tag value can only contain lowercase letters, numeric characters, and dashes.\n"+
				"Tag value must start and end with either a number or a lowercase character"))
	}
	return trace.NewAggregate(errors...)
}

// maxTagLength limits the length of the tag value.
// Tag value restrictions: see https://cloud.google.com/vpc/docs/add-remove-network-tags
const maxTagLength = 63

// tagRegex defines the syntax for tag values.
// Tag values can only contain lowercase letters, numeric characters, and dashes.
// Tag values must start and end with either a number or a lowercase character.
var tagRegex = regexp.MustCompile(`^[a-z0-9]+[a-z0-9\-]*[a-z0-9]+$`)

var tagSingleCharacterRegex = regexp.MustCompile(`^[a-z0-9]$`)
