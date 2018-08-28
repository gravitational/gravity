package utils

import . "gopkg.in/check.v1"

func (_ *UtilsSuite) TestFlattensVersion(c *C) {
	var testCases = []struct {
		input   string
		output  string
		comment string
	}{
		{
			input:   "3.1.2",
			output:  "312",
			comment: "removes punctuation",
		},
		{
			input:   "3.1.2+abc",
			output:  "312-abc",
			comment: "normalizes splitters",
		},
	}

	for _, testCase := range testCases {
		obtained := FlattenVersion(testCase.input)
		c.Assert(obtained, Equals, testCase.output, Commentf(testCase.comment))
	}
}
