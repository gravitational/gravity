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
import { UserRec } from './records';
import { List } from 'immutable';

import {
  SETTINGS_USRS_SET_NEW,
  SETTINGS_USRS_SET_CURRENT,
  SETTINGS_USRS_CLEAR,
  SETTINGS_USRS_SET_USER_TO_RESET,
  SETTINGS_USRS_SET_USER_TO_DELETE,
  SETTINGS_USRS_RECEIVE_USERS,
  SETTINGS_USRS_SET_USER_TO_REINVITE
} from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({
        users: [], 
        userToDelete: null,
        userToReset: null,
        userToReinvite: null,       
        selectedUser: null
      });
  },

  initialize() {    
    this.on(SETTINGS_USRS_SET_NEW, setNewUser);    
    this.on(SETTINGS_USRS_RECEIVE_USERS, receiveUsers);    
    this.on(SETTINGS_USRS_SET_CURRENT, setCurrent);        
    this.on(SETTINGS_USRS_CLEAR, clear);    
    this.on(SETTINGS_USRS_SET_USER_TO_RESET, (state, userId) =>
      state.set('userToReset', userId));
    this.on(SETTINGS_USRS_SET_USER_TO_DELETE, (state, userId) =>
      state.set('userToDelete', userId));
    this.on(SETTINGS_USRS_SET_USER_TO_REINVITE, (state, userId) =>
      state.set('userToReinvite', userId));
  }
})

function clear(state) {
  return state.set('selectedUser', null);
}

function setNewUser(state) {
  let user = new UserRec({
    isNew: true
  });

  return state.set('selectedUser', user); 
}

function setCurrent(state, userId) {
  let userRec = state.get('users').find(i => i.get('userId') === userId);
  return state.set('selectedUser', userRec);
}

function receiveUsers(state, json) {      
  json = json || [];
  let userList = new List( json.map( i => new UserRec(i)) ) 
  return state.setIn(['users'], userList);
}