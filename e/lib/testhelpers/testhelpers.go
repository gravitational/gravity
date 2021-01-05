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
