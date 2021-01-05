// oss imports
import cfg from 'oss-app/config'
import Settings from 'oss-app/modules/settings/components/main'
import SettingsIndex from 'oss-app/modules/settings/components/index'
import { initSettings } from 'oss-app/modules/settings/flux/actions';
import FeatureActivator from 'oss-app/modules/featureActivator';
import CertFeature from 'oss-app/modules/settings/features/certFeature';
import AccountFeature from 'oss-app/modules/settings/features/accountFeature';
import LogForwarderFeature from 'oss-app/modules/settings/features/logForwarderFeature';
import MonitorFeature from 'oss-app/modules/settings/features/monitorFeature';

// local imports
import UserFeature from './features/userFeature';
import AuthProviderFeature from './features/authConnectorFeature';
import RoleFeature from './features/roleFeature';
import './flux';

const featureActivator = new FeatureActivator();
const featureRoutes = []

featureActivator.register(new AccountFeature(featureRoutes));
featureActivator.register(new UserFeature(featureRoutes));
featureActivator.register(new AuthProviderFeature(featureRoutes));
featureActivator.register(new RoleFeature(featureRoutes));
featureActivator.register(new LogForwarderFeature(featureRoutes));
featureActivator.register(new MonitorFeature(featureRoutes));
featureActivator.register(new CertFeature(featureRoutes));

const onEnter = (nextState, replace) => {
  const { siteId } = nextState.params;
  const isRemoteAccess = siteId && cfg.isRemoteAccess(siteId);

  // if connected from ops center, redirect back to the site
  if(isRemoteAccess){
    replace(cfg.getSiteRoute(siteId));
    return;
  }

  const baseLabel = cfg.getSettingsClusterHeaderText();
  const goBackUrl = cfg.getSiteRoute(siteId);
  const baseUrl = cfg.getSiteSettingsRoute(siteId);

  const activationContext = {
    baseLabel,
    goBackUrl,
    siteId,
    baseUrl
  };

  initSettings(activationContext, featureActivator);
}

const routes = {
  title: 'Settings',
  onEnter: onEnter,
  component: Settings,
  indexRoute: {
    // need index component to handle default route
    component: SettingsIndex
  },
  childRoutes: featureRoutes
}

export default [routes];