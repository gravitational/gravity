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

package sqlite

import (
	"github.com/gravitational/trace"
	"github.com/mattn/go-sqlite3"
)

// isErrConstraintUnique returns true if error is a
// sqlite3.ErrorConstraintUnique error.
func isErrConstraintUnique(err error) bool {
	err = trace.Unwrap(err)
	sqliteErr, ok := err.(sqlite3.Error)
	if !ok {
		return false
	}
	return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
}
