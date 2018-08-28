package cli

import (
	"encoding/json"
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/modules"

	"github.com/gravitational/trace"
)

func printVersion(format constants.Format) error {
	ver := modules.Get().Version()
	switch format {
	case constants.EncodingText:
		fmt.Printf("Edition:\t%v\nVersion:\t%v\nGit Commit:\t%v\n",
			ver.Edition, ver.Version, ver.GitCommit)
	case constants.EncodingJSON:
		bytes, err := json.Marshal(ver)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf(string(bytes))
	}
	return nil
}
