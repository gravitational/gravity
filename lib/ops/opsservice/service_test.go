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

package opsservice

import (
	"net/url"
	"testing"

	"github.com/gravitational/gravity/lib/schema"
	"github.com/stretchr/testify/assert"
)

func TestGetSiteInstructionsSanitization(t *testing.T) {
	cases := []struct {
		token         string
		serverProfile string
		params        url.Values
		expectError   bool
	}{
		// Normal Inputs
		{
			token:         "token1",
			serverProfile: "profile1",
			params: map[string][]string{
				schema.AdvertiseAddr: {"1.1.1.1"},
			},
		},
		{
			token:         "token-2",
			serverProfile: "profile-2",
			params: map[string][]string{
				schema.AdvertiseAddr: {"1.1.1.1"},
			},
		},
		{
			token:         "token3_underscore",
			serverProfile: "profile3_underscore",
			params: map[string][]string{
				schema.AdvertiseAddr: {"255.255.255.255"},
			},
		},
		// Malicious Inputs
		{
			expectError:   true,
			token:         "token4;echo",
			serverProfile: "profile4",
			params: map[string][]string{
				schema.AdvertiseAddr: {"1.1.1.1"},
			},
		},
		{
			expectError:   true,
			token:         "token5$(touch grav)",
			serverProfile: "profile5",
			params: map[string][]string{
				schema.AdvertiseAddr: {"1.1.1.1"},
			},
		},
		{
			expectError:   true,
			token:         "token6",
			serverProfile: "profile6;echo",
			params: map[string][]string{
				schema.AdvertiseAddr: {"1.1.1.1"},
			},
		},
		{
			expectError:   true,
			token:         "token7",
			serverProfile: "profile7;$(touch grav)",
			params: map[string][]string{
				schema.AdvertiseAddr: {"1.1.1.1"},
			},
		},
		{
			expectError:   true,
			token:         "token8",
			serverProfile: "profile8",
			params: map[string][]string{
				schema.AdvertiseAddr: {"1.1.1.1;echo"},
			},
		},
		{
			expectError:   true,
			token:         "token9",
			serverProfile: "profile9",
			params: map[string][]string{
				schema.AdvertiseAddr: {"1.1.1.1$(touch grav)"},
			},
		},
	}

	for _, tt := range cases {
		err := validateGetSiteInstructions(tt.token, tt.serverProfile, tt.params)
		if tt.expectError {
			assert.Error(t, err, tt)
		} else {
			assert.NoError(t, err, tt)
		}

	}
}
