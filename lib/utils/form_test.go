/*
Copyright 2021 Gravitational, Inc.

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
	"bytes"
	"mime/multipart"
	"net/http"

	. "gopkg.in/check.v1"
)

type FormutilsSuite struct{}

var _ = Suite(&FormutilsSuite{})

func newRequestWithPackage(filename string) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_, err := writer.CreateFormFile("package", filename)
	if err != nil {
		return nil, err
	}
	writer.Close()
	request, err := http.NewRequest(http.MethodPost, "", body)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", writer.FormDataContentType())
	return request, nil
}

func (s *FormutilsSuite) TestParseFilename(c *C) {
	var testCases = []struct {
		filename string
		comment  string
	}{
		{
			filename: "gravitational.io/app-template:0.0.1",
			comment:  "expected original filename",
		},
		{
			filename: "test.txt",
			comment:  "expected original filename",
		},
		{
			filename: "path/to/file",
			comment:  "expected original filename",
		},
	}

	for _, testCase := range testCases {
		req, err := newRequestWithPackage(testCase.filename)
		c.Assert(err, IsNil)
		filename, err := ParseFilename(req, "package")
		c.Assert(err, IsNil)
		c.Assert(filename, Equals, testCase.filename, Commentf(testCase.comment))
	}
}
