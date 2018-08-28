
-- site_operations: remove column `provisioner`
ALTER TABLE site_operations RENAME TO _site_operations_temp;

CREATE TABLE site_operations (
  id TEXT NOT NULL PRIMARY KEY,
  account_id TEXT NOT NULL,
  site_id TEXT NOT NULL,
  type TEXT NOT NULL,
  created DATETIME NOT NULL,
  updated DATETIME NOT NULL,
  state TEXT,
  data BLOB,
  FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE,
  FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

INSERT INTO site_operations(id, account_id, site_id, type, created, updated, state, data)
SELECT id, account_id, site_id, type, created, updated, state, data
FROM _site_operations_temp;

DROP TABLE _site_operations_temp;

-- sites: rename column `provider` to `provisioner`
ALTER TABLE sites RENAME TO _sites_temp;

CREATE TABLE sites (
  id TEXT NOT NULL PRIMARY KEY,
  created DATETIME NOT NULL,
  account_id TEXT NOT NULL,
  domain_name TEXT NOT NULL UNIQUE,
  state TEXT,
  provisioner TEXT NOT NULL,
  provisioner_state TEXT,
  app_id INTEGER NOT NULL,
  FOREIGN KEY(account_id) REFERENCES accounts(id),
  FOREIGN KEY(app_id) REFERENCES apps(id)
);

INSERT INTO sites(id, created, account_id, domain_name,
		  state, provisioner, provisioner_state, app_id)
SELECT id, created, account_id, domain_name, state, provider, provisioner_state, app_id
FROM _sites_temp;

DROP TABLE _sites_temp;

-- site_with_app: reset reference `sites.provider` -> `sites.provisioner`
DROP VIEW site_with_app;

CREATE VIEW site_with_app AS
SELECT s.id, s.created, s.account_id, s.domain_name, s.state,
  s.provisioner, s.provisioner_state,
  a.repository as app_repository, a.package as app_package, a.version as app_version,
  a.type as app_type, a.manifest as app_manifest, a.namespace as app_namespace, a.info as app_info
FROM sites s JOIN apps a ON s.app_id = a.id;
