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

package utils

import (
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

// CheckEmail is a simplistic email checker
func CheckEmail(email string) error {
	if email == "" {
		return trace.BadParameter("provide a valid email address")
	}
	if !strings.Contains(email, "@") {
		return trace.BadParameter("invalid email address '%v'", email)
	}
	return nil
}

// CheckUserName validates user name
func CheckUserName(name string) error {
	if name == "" {
		return trace.BadParameter("user name cannot be empty")
	}

	return nil
}

// CheckName makes sure that the provided string is a valid app name
func CheckName(name string) error {
	if strings.TrimSpace(name) == "" {
		return trace.BadParameter("app name can't be empty")
	}
	if !nameRe.MatchString(name) {
		return trace.BadParameter("app name may contain only alphanumeric characters, dashes and underscores")
	}
	return nil
}

// nameRe is a regular expression used to validate app name
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
