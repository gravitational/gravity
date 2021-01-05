import reactor from 'app/reactor';
import store from './store';

const STORE_NAME = 'hub_nav';

reactor.registerStores({ [STORE_NAME] : store });

export const getters = {
  navStore: [STORE_NAME]
}
