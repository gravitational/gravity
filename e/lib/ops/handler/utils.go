// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func statusOK(message string) interface{} {
	return map[string]string{"status": "ok", "message": message}
}

func siteKey(p httprouter.Params) ops.SiteKey {
	return ops.SiteKey{
		AccountID:  p[0].Value,
		SiteDomain: p[1].Value,
	}
}

func rawMessage(w http.ResponseWriter, data []byte, err error) error {
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(json.RawMessage(data))
	return err
}

func message(msg string, args ...interface{}) map[string]interface{} {
	return map[string]interface{}{"message": fmt.Sprintf(msg, args...)}
}
