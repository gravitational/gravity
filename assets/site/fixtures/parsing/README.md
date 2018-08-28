These are text fixtures for parsing k8s objects from YAML/JSON.

One interesting thing about kubectl is that it allows you to mass-create
many objects from many files (from directories) or even by concatenating
several JSON definitions together like:

```json
{...}
{...}
```

... which is not a valid JSON, a proper way would be to declare an array
like this:

```json
[
  {...},
  {...}
]
```

so... this directory is a playground for parsing this stuff.
