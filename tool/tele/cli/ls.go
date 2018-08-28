package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/hub"
	"github.com/gravitational/gravity/lib/localenv"

	"github.com/dustin/go-humanize"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
)

func list(env localenv.LocalEnvironment, runtimes bool, format constants.Format) error {
	hub, err := hub.New(hub.Config{})
	if err != nil {
		return trace.Wrap(err)
	}

	items, err := hub.List()
	if err != nil {
		return trace.Wrap(err)
	}

	switch format {
	case constants.EncodingText:
		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 1, '\t', 0)
		fmt.Fprintf(w, "Name\tVersion\tCreated\tSize\n")
		fmt.Fprintf(w, "----\t-------\t-------\t----\n")
		for _, item := range items {
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\n",
				item.Name,
				item.Version,
				item.Created.Format(constants.HumanDateFormat),
				humanize.Bytes(uint64(item.SizeBytes)))
		}
		w.Flush()
	case constants.EncodingJSON:
		bytes, err := json.MarshalIndent(items, "", "    ")
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	case constants.EncodingYAML:
		bytes, err := yaml.Marshal(items)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	default:
		return trace.BadParameter("unknown output format %q, supported are: %v",
			format, constants.OutputFormats)
	}

	return nil
}
