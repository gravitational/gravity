# Gravity

## Building from source

Gravity is written in Go. There are two ways to build the Gravity tools from
source: by using locally installed build tools or via Docker.

```bash
$ git clone git@github.com:gravitational/gravity.git
$ cd gravity

# Running 'make' with the default target uses Docker.
# The output will be stored in build/<version>/
$ make

# If you have Go 1.10+ installed, you can build without Docker which is faster.
# The output will be stored in $GOPATH/bin/
$ make install

# To remove the build artifacts:
$ make clean
```
