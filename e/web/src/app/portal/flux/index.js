import reactor from 'oss-app/reactor';
import appStore from './apps/appStore';
import sitesStore from './sites/siteStore';

reactor.registerStores({
  'portal_apps': appStore,
  'portal_sites': sitesStore
});
