import React from 'react'
import { storiesOf } from '@storybook/react'
import { Roles } from './Roles'
import { RoleStoreRec } from 'e-app/cluster/flux/roles/store';
import { AccessListRec } from 'oss-app/flux/userAcl/store';

storiesOf('Gravity/Roles', module)
  .add('Roles', () => {
    const store = new RoleStoreRec().setItems(json.items);
    const userAclStore = new AccessListRec({
      authConnectors: {
        connect: true,
        list: true,
        read: true,
        edit: true,
        create: true,
      }
    });

    return (
      <Roles
        saveAttempt={{}}
        store={store}
        userAclStore={userAclStore}
      />
    );
  });

const json = {
  "items": [{
    "id": "role:@teleadmin",
    "kind": "role",
    "name": "@teleadmin",
    "displayName": "@teleadmin",
    "content": "kind: role\nmetadata:\n  labels:\n    gravitational.io/system: \"true\"\n  name: '@teleadmin'\nspec:\n  allow:\n    kubernetes_groups:\n    - admin\n    logins:\n    - root\n    node_labels:\n      '*': '*'\n    rules:\n    - resources:\n      - '*'\n      verbs:\n      - '*'\n  deny: {}\n  options:\n    cert_format: standard\n    client_idle_timeout: 0s\n    disconnect_expired_cert: false\n    forward_agent: false\n    max_session_ttl: 30h0m0s\n    port_forwarding: true\nversion: v3\n"
  }, {
    "id": "role:admin",
    "kind": "role",
    "name": "admin",
    "displayName": "admin",
    "content": "kind: role\nmetadata:\n  name: admin\nspec:\n  allow:\n    kubernetes_groups:\n    - '{{internal.kubernetes_groups}}'\n    logins:\n    - '{{internal.logins}}'\n    - root\n    node_labels:\n      '*': '*'\n    rules:\n    - resources:\n      - role\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - auth_connector\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n    - resources:\n      - session\n      verbs:\n      - list\n      - read\n    - resources:\n      - trusted_cluster\n      verbs:\n      - list\n      - create\n      - read\n      - update\n      - delete\n  deny: {}\n  options:\n    cert_format: standard\n    client_idle_timeout: 0s\n    disconnect_expired_cert: false\n    forward_agent: true\n    max_session_ttl: 30h0m0s\n    port_forwarding: true\nversion: v3\n"
  }]
}