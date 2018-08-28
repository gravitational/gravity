# Application Format

Gravity application is a tarball, i.e. any `.tar.gz` file. The file has the following structure:

```
app.tar.gz
  ├── registry
  │   └── docker
  │       └── registry
  │           └── v2 ...
  │           ... 
  └── resources
      ├── app.yaml
      └── skydns.yaml
```

Only the `registry` and `resources` directories are *mandatory*, the rest is application-specific.

In this example:
1. `registry` stores docker image layers in the `registry v2` format. See [assets/dns-app/Makefile](../../assets/dns-app/Makefile) to get an understanding about how it is produced.
2. `resources` contains the application manifest.

The rest is application-specific.
