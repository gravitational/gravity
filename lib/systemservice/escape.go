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

package systemservice

import "bytes"

// SystemdNameEscape escapes the name according to
// systemd naming convention.
// See https://www.freedesktop.org/software/systemd/man/systemd-escape.html
// for reference.
// It does not provide full implementation of systemd-escape (e.g. does not translate
// non-leading slashes to dashes and does not translate leading dot) instead it mimicks
// regular behavior of systemd for handling special characters in a unit name by
// replacing them with `\x<2-digit hex equivalent>`.
// It assumes the name to be ascii string
func SystemdNameEscape(name string) string {
	var buf bytes.Buffer

	for _, c := range name {
		switch {
		case !isValidNameChar(byte(c)):
			buf.Write(escapeChar(byte(c)))
		default:
			buf.WriteByte(byte(c))
		}
	}
	return buf.String()
}

func escapeChar(c byte) []byte {
	var result [4]byte

	result[0] = '\\'
	result[1] = 'x'
	result[2] = hexChar(c >> 4)
	result[3] = hexChar(c)
	return result[:]
}

func isValidNameChar(c byte) bool {
	return bytes.Contains(validChars, []byte{c})
}

func hexChar(c byte) byte {
	return hexChars[c&15]
}

var (
	validChars = []byte(`@:-_.\0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`)
	hexChars   = []byte("0123456789abcdef")
)
