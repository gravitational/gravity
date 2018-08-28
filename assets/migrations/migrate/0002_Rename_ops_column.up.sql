-- login_entries: rename column `portal_url` to `ops_url`
ALTER TABLE login_entries RENAME TO _login_entries_temp;

CREATE TABLE login_entries (
           user_name TEXT NOT NULL,
           ops_url TEXT NOT NULL,
           password TEXT NOT NULL,
           PRIMARY KEY(user_name, ops_url)
         );

INSERT INTO login_entries(user_name, ops_url, password)
SELECT user_name, portal_url, password
FROM _login_entries_temp;

DROP TABLE _login_entries_temp;
