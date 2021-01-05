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
