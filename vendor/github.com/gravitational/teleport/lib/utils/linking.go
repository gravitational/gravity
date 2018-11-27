/*
Copyright (c) 2013 The go-github AUTHORS. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/
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

package utils

import (
	"net/http"
	"net/url"
	"strings"
)

// WebLinks holds the pagination links parsed out of a request header
// conforming to RFC 8288.
type WebLinks struct {
	// NextPage is the next page of pagination links.
	NextPage string

	// PrevPage is the previous page of pagination links.
	PrevPage string

	// FirstPage is the first page of pagination links.
	FirstPage string

	// LastPage is the last page of pagination links.
	LastPage string
}

// ParseWebLinks partially implements RFC 8288 parsing, enough to support
// GitHub pagination links. See https://tools.ietf.org/html/rfc8288 for more
// details on Web Linking and https://github.com/google/go-github for the API
// client that this function was original extracted from.
//
// Link headers typically look like:
//
//   Link: <https://api.github.com/user/teams?page=2>; rel="next",
//     <https://api.github.com/user/teams?page=34>; rel="last"
func ParseWebLinks(response *http.Response) WebLinks {
	wls := WebLinks{}

	if links, ok := response.Header["Link"]; ok && len(links) > 0 {
		for _, lnk := range links {
			for _, link := range strings.Split(lnk, ",") {
				segments := strings.Split(strings.TrimSpace(link), ";")

				// link must at least have href and rel
				if len(segments) < 2 {
					continue
				}

				// ensure href is properly formatted
				if !strings.HasPrefix(segments[0], "<") || !strings.HasSuffix(segments[0], ">") {
					continue
				}

				// try to pull out page parameter
				link, err := url.Parse(segments[0][1 : len(segments[0])-1])
				if err != nil {
					continue
				}

				for _, segment := range segments[1:] {
					switch strings.TrimSpace(segment) {
					case `rel="next"`:
						wls.NextPage = link.String()
					case `rel="prev"`:
						wls.PrevPage = link.String()
					case `rel="first"`:
						wls.FirstPage = link.String()
					case `rel="last"`:
						wls.LastPage = link.String()
					}

				}
			}
		}
	}

	return wls
}
