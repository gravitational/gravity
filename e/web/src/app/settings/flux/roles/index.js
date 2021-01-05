import store from './store';
import reactor from 'oss-app/reactor';

const STORE_NAME = 'settings_roles';

reactor.registerStores({ [STORE_NAME] : store });

export function getRoles() {
  return reactor.evaluate([STORE_NAME]);
}

export const getters = {
  store: [STORE_NAME]
}