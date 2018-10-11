### Gravity Documentation

This Gravity documentation is public and is hosted on https://gravitational.com/gravity/docs
It's based on [MkDocs](http://www.mkdocs.org/) with our own theme, similar to
[Teleport docs](https://gravitational.com/teleport/docs/quickstart/).

### Updating

Edit markdown files, then type `make`. The Makefile uses Docker to generate
builds.

### Live Editing

You can also type `make run`. This will launch a webserver on `localhost` with
inotify-powered live updates, so you can see your markdown edits in real time
in the browser.

### Deploying

The deployment scripts for all web assets, including the documentation, are
stored in the [web repository](https://github.com/gravitational/web) which you
should clone into your `$GOPATH` side by side with the `gravity` repo.

Then you can deploy Gravity docs by running `make` commands inside `web` repo.
Read more about this in [github/gravitational/web/README.md](https://github.com/gravitational/web/blob/master/README.md)
