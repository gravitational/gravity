# magnet
Magnet is a highly experimental library utility library for working with [mage](https://magefile.org) as a build toolkit for gravitational projects.

This library mainly combines the moby/buildkit progressui components (ported to avoid vendoring all of buildkit +dependencies) with several helpers for working with docker, golang, http downloads, environment variables etc. 

# Getting Started
Get started by viewing and running the provided examples

1. [Hello World](examples/hello_world) - A simple getting started example with the UI
2. [Multi Target](examples/multi_target) - Covers multiple build targets, dependencies, and creating a hierarchy of tasks, and downloading upstream assets
3. [Docker](examples/docker) - Building and running docker containers
4. [Golang](examples/golang) - Building golang based projects