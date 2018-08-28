-- Remove FOREIGN KEY constraint on apps from sites
ALTER TABLE sites RENAME TO _sites_temp;

CREATE TABLE IF NOT EXISTS sites (
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

INSERT INTO sites SELECT * FROM _sites_temp;

DROP TABLE _sites_temp;

-- Rewrite site_with_app to deal with sites with invalid application references
-- by building a reference to a phony app
DROP VIEW site_with_app;

CREATE VIEW IF NOT EXISTS site_with_app AS
  SELECT s.id, s.created, s.account_id, s.domain_name, s.state,
    s.provider, s.provisioner_state,
    ifnull(a.repository, 'phony') AS app_repository,
    ifnull(a.package, 'unknown') AS app_package,
    ifnull(a.version, '0.0.0') AS app_version,
    ifnull(a.type, '') AS app_type,
    a.manifest AS app_manifest,
    ifnull(a.namespace, '') AS app_namespace,
    a.info as app_info
  FROM sites s LEFT JOIN apps a ON s.app_id = a.id;
