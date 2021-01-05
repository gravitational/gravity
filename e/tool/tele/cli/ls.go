package cli

import (
	"github.com/gravitational/gravity/e/lib/catalog"
	libcatalog "github.com/gravitational/gravity/lib/catalog"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/localenv"

	"github.com/gravitational/trace"
)

func list(env *localenv.LocalEnvironment, all bool, format constants.Format) error {
	lister, err := catalog.NewLister(env)
	if err != nil {
		return trace.Wrap(err)
	}
	err = libcatalog.List(lister, all, format)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
