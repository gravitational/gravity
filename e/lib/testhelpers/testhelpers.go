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

package testhelpers

import (
	"fmt"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/e/lib/ops/router"
	"github.com/gravitational/gravity/e/lib/ops/service"
	ossops "github.com/gravitational/gravity/lib/ops"
	ossclient "github.com/gravitational/gravity/lib/ops/opsclient"
	ossrouter "github.com/gravitational/gravity/lib/ops/opsroute"
	ossservice "github.com/gravitational/gravity/lib/ops/opsservice"
)

// WrapOperator wraps the provided open-source operator and returns
// the extended enterprise operator
func WrapOperator(ossOperator ossops.Operator) ops.Operator {
	switch o := ossOperator.(type) {
	case *ossservice.Operator:
		return service.New(o)
	case *ossclient.Client:
		return client.New(o)
	case *ossrouter.Router:
		return router.New(o, service.New(o.Local))
	}
	panic(fmt.Sprintf("unexpected operator type: %T", ossOperator))
}
