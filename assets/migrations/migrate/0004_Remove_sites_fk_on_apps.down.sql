-- Add FOREIGN KEY constraint on apps to sites
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
  FOREIGN KEY(account_id) REFERENCES accounts(id),
  FOREIGN KEY(app_id) REFERENCES apps(id)
);

INSERT INTO sites SELECT * FROM _sites_temp;

DROP TABLE _sites_temp;

-- Revert the site_with_app view to restore hard reference from a site to an application
DROP VIEW site_with_app;

CREATE VIEW IF site_with_app AS
  SELECT s.id, s.created, s.account_id, s.domain_name, s.state,
    s.provider, s.provisioner_state,
    a.repository as app_repository, a.package as app_package, a.version as app_version,
    a.type as app_type, a.manifest as app_manifest, a.namespace as app_namespace, a.info as app_info
  FROM sites s JOIN apps a ON s.app_id = a.id;
