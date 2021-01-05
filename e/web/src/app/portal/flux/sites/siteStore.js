import { Store, toImmutable } from 'nuclear-js';
import { PORTAL_SITE_OPEN_CONFIRM_UNLINK, PORTAL_SITE_CLOSE_CONFIRM_UNLINK   } from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({
      siteToDelete: null,
      siteToUnlink: null
    });
  },

  initialize() {    
    this.on(PORTAL_SITE_OPEN_CONFIRM_UNLINK, (state, siteId) => state.set('siteToUnlink', siteId) );
    this.on(PORTAL_SITE_CLOSE_CONFIRM_UNLINK, state => state.set('siteToUnlink', null) );
  }
})
