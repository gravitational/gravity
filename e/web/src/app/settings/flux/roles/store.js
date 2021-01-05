import { Store } from 'nuclear-js';
import * as AT from './actionTypes';
import { StoreRec } from 'oss-app/modules/settings/flux/records';

class RoleStoreRec extends StoreRec {

  constructor(props){
    super(props);
  }

  getRoleNames(){
    return super.getItems().map(r => r.getName()).toJS();
  }
}

export default Store({

  getInitialState() {
    return new RoleStoreRec()
  },

  initialize() {
    this.on(AT.UPSERT_ROLES, (state, items) => state.upsertItems(items) );
    this.on(AT.RECEIVE_ROLES, (state, items) => state.setItems(items) );
    this.on(AT.SET_CURRENT, (state, item) => state.setCurItem(item))
    this.on(AT.DELETE_ROLE, (state, id) => state.remove(id))
  }
})

