# Gravity Docs

Gravity docs are built using [mkdocs](http://www.mkdocs.org/) and hosted as static files
on CloudFlare CDN. Located here https://gravitational.com/gravity/docs/overview/

Look at the `Makefile` to see how it works.

## Adding a New Version

To add a new version, say 8.x:

* copy 7.x.yaml to 8.x.yaml
* edit 8.x.yaml
* edit theme/scripts.html and update docVersions variable


## Deploying

Gravity docs are published using the private `web `repository
See `web/README.md` for more info.

## Running Locally

We recommend using Docker to run and build the docs.

`make run` will create a build a local Docker environment, compile the docs and
setup a [livereload server](https://chrome.google.com/webstore/detail/livereload/jnihajbhpnppcggbcgedagnkighmdlei?hl=en).
Access this at http://localhost:6601/overview/ in your local browser.

By default, `make run` will run the latest version of the docs. This can be overridden by
specifying `RUN_CFG=5.x.yaml`. For example:

```
make run RUN_CFG=4.x.yaml
```

`make docs` will build the docs, so they are ready to ship to production.

foo
