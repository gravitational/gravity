package main

import (
	"github.com/gravitational/gravity/lib/terraform/provider"

	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: provider.Provider})
}
