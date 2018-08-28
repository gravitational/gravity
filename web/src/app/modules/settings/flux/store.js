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

import {Store} from 'nuclear-js';
import {Record, List, OrderedMap} from 'immutable';
import {INIT, ADD_NAV_ITEM} from './actionTypes';
import reactor from 'app/reactor';
import sitesGetters from 'app/flux/sites/getters';

const ensureNotEmptyList = arary => arary ? arary : List();

class SettingsStore extends Record({
  siteId: null,
  goBackUrl: '',
  baseUrl: '',
  baseLabel: '',
  navGroups: new OrderedMap()
}) {

  constructor(props) {
    super(props);
  }

  getClusterName(){
    return this.get('siteId');
  }

  getLogoUri(){
    return reactor.evaluate(sitesGetters.siteLogo(this.siteId));
  }

  getNavGroup(groupName){
    const items = this.getIn(['navGroups', groupName]);
    return items ? items.toJS() : [];
  }

  addNavItem({groupName, navItem}) {
    let navGroup = this.getIn(['navGroups', groupName]);
    navGroup = ensureNotEmptyList(navGroup);
    navGroup = navGroup.push(navItem);
    return this.setIn([ 'navGroups', groupName ], navGroup)
  }
}

export default Store({
  getInitialState() {
    return new SettingsStore();
  },

  initialize() {
    this.on(INIT, (state, newState) => new SettingsStore(newState))
    this.on(ADD_NAV_ITEM, (state, props) => state.addNavItem(props))
  }
})
