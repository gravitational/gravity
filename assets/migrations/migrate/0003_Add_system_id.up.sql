-- add new columns to site_operation_servers to describe host operating system: os_id, os_version
ALTER TABLE site_operation_servers RENAME TO _site_operation_servers_temp;

CREATE TABLE IF NOT EXISTS site_operation_servers (
  operation_id TEXT NOT NULL,
  advertise_ip TEXT NOT NULL,
  hostname TEXT,
  role TEXT,
  os_id TEXT,
  os_version TEXT,
  PRIMARY KEY(operation_id, advertise_ip),
  FOREIGN KEY(operation_id) REFERENCES site_operations(id) ON DELETE CASCADE
);

INSERT INTO site_operation_servers(operation_id, advertise_ip, hostname, role)
SELECT operation_id, advertise_ip, hostname, role FROM _site_operation_servers_temp;

DROP TABLE _site_operation_servers_temp;
