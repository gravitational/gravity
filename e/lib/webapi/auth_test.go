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
