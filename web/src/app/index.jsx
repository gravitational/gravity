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

import React from 'react';
import { Router } from 'react-router';
import { Provider } from 'nuclear-js-react-addons';
import DocumentTitle from './components/common/documentTitle';
import reactor from './reactor';
import history from './services/history';
import cfg from './config';
import App from './components/app';
import { ensureUser } from './flux/user/actions';
import * as Messages from './components/msgPage';
import UserLogin from './components/user/login';
import UserInviteReset from './components/user/userInviteReset';
import { hot } from 'react-hot-loader'

import './vendor';
import './../styles/index.scss';
import './flux';

let rootRoutes = [
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
            path: cfg.routes.installerBase,
            onEnter: ensureUser,
            getChildRoutes: (nextState, cb) => cb(null, require('./modules/installer').default)
          },
          {
            path: cfg.routes.siteBase,
            onEnter: ensureUser,
            getChildRoutes: (nextState, cb) => cb(null, require('./modules/site').default)
          },
          {
            path: cfg.routes.siteSettings,
            onEnter: ensureUser,
            getChildRoutes: (nextState, cb) => cb(null, require('./modules/settings').default)
          },
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
if (module.hot) {
  module.hot.accept('./modules/installer', () => {
    require('./modules/installer');
  })

  module.hot.accept('./modules/site', () => {
    require('./modules/site');
  })

  module.hot.accept('./modules/settings', () => {
    require('./modules/settings');
  })
}

export default hot(module)(Root);