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
import { OPS_RECEIVE, OPS_ADD, OPS_DELETE } from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(OPS_RECEIVE, receiveOps);
    this.on(OPS_ADD, addOp);
    this.on(OPS_DELETE, (state, opId) => state.delete(opId));
  }
})

function receiveOps(state, opDataArray){
  return toImmutable({}).withMutations(state => {
    opDataArray.forEach( item => addOp(state, item) )
  })
}

function addOp(state, item){
  let itemMap = toImmutable(item);
  let siteId = itemMap.get('site_domain');
  let data = null;

  data = itemMap.get('shrink');
  if(!data){
    data = itemMap.get('install_expand')
  }

  if(!data){
    data = itemMap.get('uninstall')
  }

  if(!data){
    data = itemMap.get('update')
  }

  itemMap = itemMap.set('site_id', siteId)
                   .set('data', data)
                   .set('created', new Date(item.created));

  return state.set(item.id, itemMap)

}
