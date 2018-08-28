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

import $ from 'jQuery';
import cfg from 'app/config'
import FeatureBase from '../../featureBase'
import SiteServerPage from './../components/serverPage/main.jsx';
import {MasterConsoleActivator} from './../components/masterConsolePage/main.jsx';
import {onEnterTerminalPage, onLeaveTerminalPage} from './../flux/masterConsole/actions';
import {addNavItem} from './../flux/currentSite/actions';
import * as initActionTypes from './actionTypes';
import * as serverActions from './../flux/servers/actions';

class ServersFeature extends FeatureBase {

  constructor(routes) {
    super(initActionTypes.SERVERS);
    routes.push({
      path: cfg.routes.siteServers,      
      indexRoute: {
        component: super.withMe(SiteServerPage)
      },
      childRoutes: [
        {
          path: cfg.routes.siteConsole,
          onEnter: onEnterTerminalPage,
          onLeave: onLeaveTerminalPage,
          component: MasterConsoleActivator
        }
      ]
    })
  }

  onload(context) {
    this.startProcessing();
    $.when(serverActions.fetchServers())
      .done(this.activate.bind(this))
      .fail(this.handleError.bind(this));    

    addNavItem({
      title: 'Servers',
      key: 'servers',
      icon: 'fa fa-server',
      to: cfg.getSiteServersRoute(context.siteId),
      children: []
    })
  }

  activate(){
    try {
      this.stopProcessing()
    } catch (err) {
      this.handleError(err)
    }
  }
}

export default ServersFeature