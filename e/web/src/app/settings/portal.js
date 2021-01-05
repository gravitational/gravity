/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// oss imports
import Settings from 'oss-app/modules/settings/components/main'
import SettingsIndex from 'oss-app/modules/settings/components/index'
import FeatureActivator from 'oss-app/modules/featureActivator';
import CertFeature from 'oss-app/modules/settings/features/certFeature';
import AccountFeature from 'oss-app/modules/settings/features/accountFeature';
import { initSettings } from 'oss-app/modules/settings/flux/actions';

// local imports
import AuthProviderFeature from './features/authConnectorFeature';
import RoleFeature from './features/roleFeature';
import LicenseFeature from './features/licenseFeature';
import UserFeature from './features/userFeature';
import cfg from './../config';
import './flux';

const featureActivator = new FeatureActivator();
const featureRoutes = []

featureActivator.register(new AccountFeature(featureRoutes));
featureActivator.register(new UserFeature(featureRoutes));
featureActivator.register(new AuthProviderFeature(featureRoutes));
featureActivator.register(new RoleFeature(featureRoutes));
featureActivator.register(new LicenseFeature(featureRoutes));
featureActivator.register(new CertFeature(featureRoutes));

const onEnter = () => {
  const siteId = cfg.getLocalSiteId();
  const baseUrl = cfg.routes.portalSettings;
  const baseLabel = cfg.getSettingsOpsCenterHeaderText();
  const goBackUrl = cfg.routes.portalBase;
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