/*
Copyright 2019 Gravitational, Inc.

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

// Package fields provides methods for making log fields from various objects.
package fields

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

// FromRequest makes fields from the provided HTTP request.
func FromRequest(r *http.Request) logrus.Fields {
	return logrus.Fields{"method": r.Method, "url": r.URL.Path}
}
