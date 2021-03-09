# Contributing

Gravity is an open source project.

It is the work of tens of contributors. We appreciate your help!

We follow a [code of conduct](./CODE_OF_CONDUCT.md).


## Filing an Issue

Security issues should be reported directly to security@gravitational.com.

If you are unsure if you've found a bug, consider searching
[current issues](https://github.com/gravitational/gravity/issues) or
asking in the [community forum](https://community.goteleport.com/) first.

Once you know you have a issue, make sure to fill out all sections of the
one of the templates at https://github.com/gravitational/gravity/issues/new/choose.

Gravity contributors will triage the issue shortly.


## Contributing A Patch

If you're working on an existing issue, respond to the issue and express
interest in working on it. This helps other people know that the issue is
active, and hopefully prevents duplicated efforts.

If you want to work on a new idea of relatively small scope:

1. Submit an issue describing the proposed change and the implementation.
2. The repo owners will respond to your issue promptly.
3. Write your code, test your changes and _communicate_ with us as you're
moving forward.
4. Submit a pull request from your fork.

### Adding dependencies

If your patch depends on new packages, the dependencies must:

- be licensed via Apache2 license
- be approved by core Gravity contributors ahead of time
- be vendored via go modules
