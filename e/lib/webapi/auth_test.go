// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBrowserURL(t *testing.T) {
	cases := []struct {
		url       string
		expectErr bool
	}{
		// No error - regular http/https URLs
		{
			url:       "https://gravitational.io",
			expectErr: false,
		},
		{
			url:       "http://gravitational.io",
			expectErr: false,
		},
		{
			url:       "HTTP://gravitational.io",
			expectErr: false,
		},

		{
			url:       "HTTPs://gravitational.io",
			expectErr: false,
		},
		// Expect error on non-nttp(s) urls and unparseable urls
		{
			url:       "file://gravitational.io",
			expectErr: true,
		},
		{
			url:       "file:///Applications/Calculator.app",
			expectErr: true,
		},
		{
			url:       "fileishere",
			expectErr: true,
		},
		{
			url:       "chrome://example",
			expectErr: true,
		},
		{
			url:       "example://example",
			expectErr: true,
		},
		{
			url:       "example://example",
			expectErr: true,
		},
	}

	for _, tt := range cases {
		err := validateBrowserURL(tt.url)
		if tt.expectErr {
			assert.Error(t, err, tt.url)
		} else {
			assert.NoError(t, err, tt.url)
		}
	}
}
