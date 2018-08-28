
## Database schema migrations

We have rudimentary support for migrating schemas (and data) between versions using [migrate].

### Setup

Build gravity to make sure you're using the up-to-date version.

### Running migrations

Upgrade a database to the current head:

```shell
$ cd assets/migrations
$ gravity migrate up --url=sqlite3:///var/lib/gravity/gravity.db ./migrate
```
Use `down` for downgrade:

```shell
$ gravity migrate down --url=sqlite3:///var/lib/gravity/gravity.db ./migrate
```

A single migration step consists of two files - one for the upgrade and one for the reverse:

```
0001_Add_provision_provider_columns.down.sql
0001_Add_provision_provider_columns.up.sql
```

Each file has a schema version embedded in the name and the files are applied in ascending order of the version attribute.

To facilitate migration management, gravity stamps the schema by embedding the current schema version when the database is created.
lib/defaults/defaults#DatabaseSchemaVersion defines the current schema version to use for stamping:

```go
	// SchemaVersion is a running counter for the current version of the database schema.
	// The version is used when generating an empty database as a stamp for a subsequent migration step.
	// It is important to keep the schema version up-to-date with the tip version of the migration state.
	DatabaseSchemaVersion = 1
```

It is important to keep this field in sync whenever a new migration step is added and schema version gets incremented.


[//]: # (Footnotes and references)

[migrate]: <https://github.com/mattes/migrate>
