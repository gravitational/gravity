import { Store, toImmutable } from 'nuclear-js';
import {
  PORTAL_APPS_OPEN_CONFIRM_DELETE,
  PORTAL_APPS_CLOSE_CONFIRM_DELETE } from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({
      appToDelete: null
    });
  },

  initialize() {
    this.on(PORTAL_APPS_OPEN_CONFIRM_DELETE, openAppConfirmDialog);
    this.on(PORTAL_APPS_CLOSE_CONFIRM_DELETE, closeAppConfirmDialog);
  }
})

function openAppConfirmDialog(state, appId){
  return state.set('appToDelete', appId);
}

function closeAppConfirmDialog(state){
  return state.set('appToDelete', null);
}
