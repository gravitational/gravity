/*
Copyright 2018 Gravitational, Inc.

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
import { RECEIVE_NODES } from './actionTypes';

export class ServerRec extends Record({
  id: '',
  siteId: '',
  hostname: '',
  tags: new List(),
  addr: '',
  role: '',
  ip: '',
}) {
  constructor(props) {
    const tags = new List(toImmutable(props.tags));
    const ipLabel = tags.find(t => t.get('name') === 'advertise-ip');
    const roleLabel = tags.find(t => t.get('name') === 'role');
    const ip = ipLabel ? ipLabel.get('value') : null;
    const role = roleLabel ? roleLabel.get('value') : null;
    super({
      ...props,
      ip,
      role,
    })
  }
}

class NodeStoreRec extends Record({
  servers: new List()
}) {

  findServer(serverId) {
    return this.servers.find(s => s.id === serverId);
  }

  getServers(siteId) {
    return this.servers.filter(s => s.siteId === siteId);
  }

  addServers(jsonItems) {
    const list = new List().withMutations(state => {
      jsonItems.forEach(item => state.push(new ServerRec(item)));
      return state;
    });

    return list.equals(this.servers) ? this : this.set('servers', list);
  }
}

export default Store({
  getInitialState() {
    return new NodeStoreRec();
  },

  initialize() {
    this.on(RECEIVE_NODES, (state, items) => state.addServers(items))
  }
})
