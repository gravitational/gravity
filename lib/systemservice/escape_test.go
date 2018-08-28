package systemservice

import (
	. "gopkg.in/check.v1"
)

func (_ *SystemdSuite) TestEscapesSystemdUnitNames(c *C) {
	var testCases = []struct {
		input    string
		expected string
		comment  string
	}{
		{"foo+-!.service", `foo\x2b-\x21.service`, "transforms characters outside allowed set"},
		{".foo_BAR-baz.service", ".foo_BAR-baz.service", "no escaping necessary"},
	}

	for _, testCase := range testCases {
		result := SystemdNameEscape(testCase.input)
		c.Assert(result, DeepEquals, testCase.expected, Commentf(testCase.comment))
	}
}
