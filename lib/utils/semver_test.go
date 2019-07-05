/*
Copyright 2019 Gravitational, Inc.

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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coreos/go-semver/semver"
)

func TestSemverSanity(t *testing.T) {
	cases := []struct {
		version   semver.Version
		shouldErr bool
	}{
		//
		// Normal Inputs
		//
		{
			version:   *semver.New("0.0.0"),
			shouldErr: false,
		},
		{
			version:   *semver.New("0.0.0-alpha.1"),
			shouldErr: false,
		},
		{
			version:   *semver.New("0.0.0-alpha.0"),
			shouldErr: false,
		},
		{
			version:   *semver.New("99.0.0-alpha.106"),
			shouldErr: false,
		},
		{
			version:   *semver.New("0.0.0+some-text"),
			shouldErr: false,
		},
		{
			version:   *semver.New("0.0.0-alpha.55+some-text-Plus-Uppercase"),
			shouldErr: false,
		},
		//
		// Malicious Inputs
		//
		{
			version:   *semver.New("0.0.0+;echo"),
			shouldErr: true,
		},
		{
			version:   *semver.New("0.0.0-;echo"),
			shouldErr: true,
		},
		{
			version:   *semver.New("1.0.1-aaa$(touch grav)"),
			shouldErr: true,
		},
		{
			version:   *semver.New("1.0.1+aaa$(touch grav)"),
			shouldErr: true,
		},
	}

	for _, tt := range cases {
		if tt.shouldErr {
			assert.Error(t, SanitizeSemver(tt.version), tt.version.String())
		} else {
			assert.NoError(t, SanitizeSemver(tt.version), tt.version.String())
		}
	}
}
