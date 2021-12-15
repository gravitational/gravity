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
	"mime"
	"net/http"

	"github.com/gravitational/trace"
)

const (
	defaultMaxMemory = 32 << 20 // 32 MB
)

// ParseFilename parses the filename for the specified form data.
func ParseFilename(req *http.Request, key string) (string, error) {
	if err := req.ParseMultipartForm(defaultMaxMemory); err != nil {
		return "", trace.Wrap(err)
	}
	if req.MultipartForm == nil {
		return "", trace.BadParameter("request does not contain multipart form")
	}
	fileHeaders, exists := req.MultipartForm.File[key]
	if !exists {
		return "", trace.NotFound("multipart form does not contain %s data", key)
	}
	if len(fileHeaders) != 1 {
		return "", trace.BadParameter("expected a single file parameter but got %d", len(fileHeaders))
	}
	_, params, _ := mime.ParseMediaType(fileHeaders[0].Header.Get("Content-Disposition"))
	filename, exists := params["filename"]
	if !exists {
		return "", trace.NotFound("file header does not contain filename")
	}
	return filename, nil
}
