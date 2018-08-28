-- app_operations: make the ID a text, connect with repository_packages
ALTER TABLE app_operations RENAME TO _app_operations_temp;

CREATE TABLE app_operations (
  id TEXT NOT NULL PRIMARY KEY,
  type TEXT NOT NULL,
  created DATETIME NOT NULL,
  updated DATETIME NOT NULL,
  state TEXT,
  repository_name TEXT NOT NULL,
  package_name TEXT NOT NULL,
  package_version TEXT NOT NULL
);

INSERT INTO app_operations(id, type, created, updated, state,
                           repository_name, package_name, package_version)
SELECT ops.id, ops.type, ops.created, ops.updated, ops.state,
       a.repository, a.package, a.version
FROM _app_operations_temp ops JOIN apps a ON ops.app_id = a.id
UNION 
SELECT ops.id, 'operation_app_import' AS type, ops.created, ops.updated, ops.state,
       a.repository, a.package, a.version
FROM app_import_operations ops JOIN apps a ON ops.repository = a.repository
  AND ops.package = a.package AND ops.version = a.version;

DROP TABLE _app_operations_temp;

-- app_progress_entries: operation ID column is text
ALTER TABLE app_progress_entries RENAME TO _app_progress_entries_temp;

CREATE TABLE IF NOT EXISTS app_progress_entries (
  operation_id TEXT NOT NULL,
  created DATETIME NOT NULL,
  completion INTEGER NOT NULL,
  state TEXT NOT NULL,
  message TEXT NOT NULL,
  FOREIGN KEY(operation_id) REFERENCES app_operations(id) ON DELETE CASCADE
);

INSERT INTO app_progress_entries(operation_id, created, completion, state, message)
SELECT ops.id, p.created, p.completion, p.state, p.message
FROM _app_progress_entries_temp p JOIN app_operations ops ON p.operation_id = ops.id
UNION
SELECT ops.id, p.created, p.completion, p.state, p.message
FROM app_import_progress_entries p JOIN app_import_operations ops ON p.operation_id = ops.id;

DROP TABLE _app_progress_entries_temp;

-- app_import_operations: drop table
DROP TABLE app_import_operations;

-- repository_packages: merge with `apps` table
ALTER TABLE repository_packages RENAME TO _repository_packages_temp;

CREATE TABLE repository_packages (
  repository_name TEXT NOT NULL,
  name TEXT NOT NULL,
  version TEXT NOT NULL,
  sha512 TEXT NOT NULL,
  size_bytes INT,
  hidden BOOLEAN DEFAULT 0,
  -- application packages: user/service
  type TEXT,
  manifest BLOB,
  PRIMARY KEY(repository_name, name, version),
  FOREIGN KEY(repository_name) REFERENCES repositories(name) ON DELETE CASCADE
);

INSERT INTO repository_packages(repository_name, name, version, sha512, size_bytes, hidden, type, manifest)
SELECT p.repository_name, p.name, p.version, p.sha512, p.size_bytes, 0 AS hidden, NULL AS type, NULL AS manifest
FROM _repository_packages_temp p LEFT JOIN apps a ON p.repository_name = a.repository AND p.name = a.package AND p.version = a.version
WHERE a.repository IS NULL AND a.package IS NULL AND a.version IS NULL
UNION
SELECT p.repository_name, p.name, p.version, p.sha512, p.size_bytes,
       case a.type
        when 'service' then 1
        else 0
       end AS hidden,
       a.type, a.manifest AS manifest
FROM _repository_packages_temp p JOIN apps a ON p.repository_name = a.repository AND p.name = a.package AND p.version = a.version;

DROP TABLE _repository_packages_temp;

-- connect sites with packages
ALTER TABLE sites RENAME TO _sites_temp;

CREATE TABLE sites (
  id TEXT NOT NULL PRIMARY KEY,
  created DATETIME NOT NULL,
  account_id TEXT NOT NULL,
  domain_name TEXT NOT NULL UNIQUE,
  state TEXT,
  provider TEXT NOT NULL,
  provisioner_state TEXT,
  repository_name TEXT,
  package_name TEXT,
  package_version TEXT,
  FOREIGN KEY(account_id) REFERENCES accounts(id)
);

INSERT INTO sites(id, created, account_id, domain_name, state, provider, provisioner_state,
                  repository_name, package_name, package_version)
SELECT s.id, s.created, s.account_id, s.domain_name, s.state, s.provider, s.provisioner_state,
       IFNULL(a.repository, 'phony'), IFNULL(a.package, 'unknown'), IFNULL(a.version, '0.0.0')
FROM _sites_temp s LEFT JOIN apps a ON s.app_id = a.id;

DROP TABLE _sites_temp;

-- app_import_progress_entries: drop table
DROP TABLE app_import_progress_entries;

-- apps: drop table
DROP TABLE apps;

-- site_with_app: drop view
DROP VIEW site_with_app;
