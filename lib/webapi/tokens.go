package webapi

import (
	"net/http"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// generateInstallToken creates a new one-time installation token
//
// POST /portalapi/v1/tokens/install
//
// Input:
// {
//   "app": "gravitational.io/k8s-aws:1.15.0-138"
// }
//
// Output:
// {
//     "token": "value",
//     "expires": "RFC3339 expiration timestamp",
//     "account_id": "account-id",
//     "user_email": "agent@domain"
// }
func (m *Handler) generateInstallToken(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	var req ops.NewInstallTokenRequest
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	req.AccountID = ctx.User.GetAccountID()
	req.UserType = storage.AgentUser
	token, err := ctx.Operator.CreateInstallToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}
