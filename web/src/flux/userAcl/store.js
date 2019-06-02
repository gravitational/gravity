/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { Store, toImmutable } from 'nuclear-js';
import { Record, List } from 'immutable';
import { USERACL_RECEIVE } from './actionTypes';

// sort logins by making 'root' as the first in the list
const sortLogins = loginList => {
  let index = loginList.indexOf('root');
  if (index !== -1) {
    loginList = loginList.remove(index);
    return loginList.sort().unshift('root')
  }

  return loginList;
}

const Access = new Record({
  connect: false,
  list: false,
  read: false,
	edit: false,
	create: false,
	remove: false
})

export class AccessListRec extends Record({
  authConnectors: new Access(),
  trustedClusters: new Access(),
  roles: new Access(),
  sessions: new Access(),
  licenses: new Access(),
  clusters: new Access(),
  repositories: new Access(),
  users: new Access(),
  logForwarders: new Access(),
  sshLogins: new List()
}){
  constructor(json = {}) {
    const map = toImmutable(json);
    const sshLogins = new List(map.get('sshLogins'));
    const params = {
      sshLogins: sortLogins(sshLogins),
      authConnectors: new Access(map.get('authConnectors')),
      trustedClusters: new Access(map.get('trustedClusters')),
      roles: new Access(map.get('roles')),
      sessions: new Access(map.get('sessions')),
      users: new Access(map.get('users')),
      licenses: new Access(map.get('licenses')),
      clusters: new Access(map.get('clusters')),
      logForwarders: new Access(map.get('logForwarders'))
    }

    super(params);
  }

  getClusterAccess(){
    return this.get('clusters');
  }

  getLicenseAccess(){
    return this.get('licenses');
  }

  getSessionAccess() {
    return this.get('sessions');
  }

  getRoleAccess() {
    return this.get('roles');
  }

  getUserAccess() {
    return this.get('users');
  }

  getConnectorAccess() {
    return this.get('authConnectors');
  }

  getLogForwarderAccess() {
    return this.get('logForwarders');
  }

  getSshLogins() {
    return this.get('sshLogins')
  }
}

export default Store({
  getInitialState() {
    return new AccessListRec();
  },

  initialize() {
    this.on(USERACL_RECEIVE, (state, json ) => new AccessListRec(json) );
  }
})