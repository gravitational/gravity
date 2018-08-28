-- apps: restore
CREATE TABLE apps (
  id INTEGER PRIMARY KEY NOT NULL,
  repository TEXT NOT NULL,
  package TEXT NOT NULL,
  version TEXT NOT NULL,
  -- user/service
  type TEXT NOT NULL,
  namespace TEXT,
  manifest BLOB,
  info BLOB,
  UNIQUE(repository, package, version),
  FOREIGN KEY(repository, package, version) REFERENCES repository_packages(repository_name, name, version) ON DELETE CASCADE
);

INSERT INTO apps(repository, package, version, type, manifest)
SELECT repository_name, name, version, type, manifest
FROM repository_packages WHERE manifest IS NOT NULL;

-- app_import_operations: restore
CREATE TABLE app_import_operations (
  id INTEGER PRIMARY KEY NOT NULL,
  repository TEXT NOT NULL,
  package TEXT NOT NULL,
  version TEXT NOT NULL,
  created DATETIME NOT NULL,
  updated DATETIME NOT NULL,
  state TEXT
);

INSERT INTO app_import_operations(id, repository, package, version, created, updated, state)
SELECT ops.id, p.repository_name, p.name, p.version, ops.created, ops.updated, ops.state
FROM app_operations ops JOIN repository_packages p ON ops.repository_name = p.repository_name
  AND ops.package_name = p.name AND ops.package_version = p.version
WHERE ifnull(ops.type, '') = 'operation_app_import';

-- app_import_progress_entries: restore
CREATE TABLE app_import_progress_entries (
  operation_id INTEGER NOT NULL,
  created DATETIME NOT NULL,
  completion INTEGER NOT NULL,
  state TEXT NOT NULL,
  message TEXT NOT NULL,
  FOREIGN KEY(operation_id) REFERENCES app_import_operations(id) ON DELETE CASCADE
);

INSERT INTO app_import_progress_entries(operation_id, created, completion, state, message)
SELECT ops.id, e.created, e.completion, e.state, e.message
FROM app_progress_entries e JOIN app_operations ops ON ops.id = e.operation_id
WHERE IFNULL(ops.type, '') = 'operation_app_import';

-- app_progress_entries: revert the operation ID to integer
ALTER TABLE app_progress_entries RENAME TO _app_progress_entries_temp;

CREATE TABLE app_progress_entries (
  app_id INTEGER NOT NULL,
  operation_id INTEGER NOT NULL,
  created DATETIME NOT NULL,
  completion INTEGER NOT NULL,
  state TEXT NOT NULL,
  message TEXT NOT NULL,
  FOREIGN KEY(app_id) REFERENCES apps(id) ON DELETE CASCADE,
  FOREIGN KEY(operation_id) REFERENCES app_operations(id) ON DELETE CASCADE
);

INSERT INTO app_progress_entries(app_id, operation_id, created, completion, state, message)
SELECT a.id, ops.id, e.created, e.completion, e.state, e.message
FROM app_progress_entries e JOIN app_operations ops ON ops.id = e.operation_id
  JOIN apps a ON a.repository = ops.repository_name AND a.package = ops.package_name
    AND a.version = ops.package_version
WHERE ops.type IS NULL OR ops.type != 'operation_app_import';

DROP TABLE _app_progress_entries_temp;

-- app_operations: revert the ID to integer, connect with apps
ALTER TABLE app_operations RENAME TO _app_operations_temp;

CREATE TABLE app_operations (
  id INTEGER PRIMARY KEY NOT NULL,
  app_id INTEGER NOT NULL,
  type TEXT NOT NULL,
  created DATETIME NOT NULL,
  updated DATETIME NOT NULL,
  state TEXT,
  FOREIGN KEY(app_id) REFERENCES apps(id) ON DELETE CASCADE
);

INSERT INTO app_operations(id, app_id, created, updated, state)
SELECT ops.id, a.id, ops.created, ops.updated, ops.state
FROM _app_operations_temp ops JOIN repository_packages p ON ops.repository_name = p.repository_name
  AND ops.package_name = p.name AND ops.package_version = p.version
JOIN apps a ON a.repository = p.repository_name AND a.package = p.name
  AND a.version = p.version
WHERE ops.type IS NULL OR ops.type != 'operation_app_import';

DROP TABLE _app_operations_temp;

-- repository_packages: drop application-specific columns
ALTER TABLE repository_packages RENAME TO _repository_packages_temp;

CREATE TABLE repository_packages (
  repository_name TEXT NOT NULL,
  name TEXT NOT NULL,
  version TEXT NOT NULL,
  sha512 TEXT NOT NULL,
  size_bytes INT,
  PRIMARY KEY(repository_name, name, version),
  FOREIGN KEY(repository_name) REFERENCES repositories(name) ON DELETE CASCADE
);

INSERT INTO repository_packages(repository_name, name, version, sha512, size_bytes)
SELECT p.repository_name, p.name, p.version, p.sha512, p.size_bytes
FROM _repository_packages_temp p;

DROP TABLE _repository_packages_temp;

-- sites: reconnect with `apps`
ALTER TABLE sites RENAME TO _sites_temp;

CREATE TABLE sites (
  id TEXT NOT NULL PRIMARY KEY,
  created DATETIME NOT NULL,
  account_id TEXT NOT NULL,
  domain_name TEXT NOT NULL UNIQUE,
  state TEXT,
  provider TEXT NOT NULL,
  provisioner_state TEXT,
  app_id INTEGER NOT NULL,
  FOREIGN KEY(account_id) REFERENCES accounts(id)
);

INSERT INTO sites(id, created, account_id, domain_name, state, provider, provisioner_state, app_id)
SELECT s.id, s.created, s.account_id, s.domain_name, s.state, s.provider, s.provisioner_state, a.id
FROM _sites_temp s LEFT JOIN apps a ON s.repository_name = a.repository
  AND s.package_name = a.package AND s.package_version = a.version;

DROP TABLE _sites_temp;

-- site_with_app: restore
CREATE VIEW site_with_app AS
  SELECT s.id, s.created, s.account_id, s.domain_name, s.state,
    s.provider, s.provisioner_state,
    IFNULL(a.repository, 'phony') AS app_repository,
    IFNULL(a.package, 'unknown') AS app_package,
    IFNULL(a.version, '0.0.0') AS app_version,
    a.type AS app_type, a.manifest AS app_manifest, a.namespace AS app_namespace,
    a.info as app_info
  FROM sites s JOIN apps a ON s.app_id = a.id;
