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
	validChars []byte = []byte(`@:-_.\0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`)
	hexChars   []byte = []byte("0123456789abcdef")
)
