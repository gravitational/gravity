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

import React from 'react';
import { Router } from 'react-router';
import { Provider } from 'nuclear-js-react-addons';
import { hot } from 'react-hot-loader'

// oss imports
import DocumentTitle from 'oss-app/components/common/documentTitle';
import reactor from 'oss-app/reactor';
import history from 'oss-app/services/history';
import App from 'oss-app/components/app';
import { ensureUser } from 'oss-app/flux/user/actions';
import * as Messages from 'oss-app/components/msgPage';
import UserLogin from 'oss-app/components/user/login';
import UserInviteReset from 'oss-app/components/user/userInviteReset';
import 'oss-app/vendor';
import 'oss-app/flux';

// local imports
import cfg from './config';
import './../styles/index.scss';

const rootRoutes = [
  /*
  * The <Redirect> configuration helper is not available when using plain routes,
  * so you have to set up the redirect using the onEnter hook.
  **/
  {
    component: DocumentTitle,
    childRoutes: [
      { path: cfg.routes.app, onEnter: (localtion, replace) => replace(cfg.routes.defaultEntry) },
      { path: cfg.routes.logout, onEnter: (localtion, replace) => replace(cfg.routes.login) },
      { path: cfg.routes.errorPage, component: Messages.ErrorPage },
      { path: cfg.routes.infoPage, component: Messages.InfoPage },
      {
        path: cfg.routes.app, component: App,
        childRoutes: [
          { path: cfg.routes.login, title: "Login", component: UserLogin },
          { path: cfg.routes.userInvite, title: "Invite", component: UserInviteReset },
          { path: cfg.routes.userReset, component: UserInviteReset },
          {
            path: cfg.routes.portalBase,
            onEnter: ensureUser,
            getChildRoutes: (nextState, cb) => cb(null, require('./portal').default)
          },
          {
            path: cfg.routes.installerBase,
            onEnter: ensureUser,
            getChildRoutes: (nextState, cb) => cb(null, require('oss-app/modules/installer').default)
          },
          {
            path: cfg.routes.siteBase,
            onEnter: ensureUser,
            getChildRoutes: (nextState, cb) => cb(null, require('oss-app/modules/site').default)
          },
          {
            path: cfg.routes.siteSettings,
            onEnter: ensureUser,
            getChildRoutes: (nextState, cb) => cb(null, require('./settings/cluster').default)
          },
          {
            path: cfg.routes.portalSettings,
            onEnter: ensureUser,
            getChildRoutes: (nextState, cb) => cb(null, require('./settings/portal').default)
          }
        ]
      },
      { path: '*', component: Messages.NotFound }
    ]
  }
];

const Root = () => (
  <Provider reactor={reactor}>
    <Router history={history.original()} routes={rootRoutes} />
  </Provider>
)

// enable hot reloading
if(module.hot){
  module.hot.accept('oss-app/modules/site', () => {
    require('oss-app/modules/site');
  })

  module.hot.accept('./settings/cluster', () => {
    require('./settings/cluster');
  })

  module.hot.accept('./settings/portal', () => {
    require('./settings/portal');
  })

  module.hot.accept('./portal', () => {
    require('./portal');
  })
}

export default hot(module)(Root);