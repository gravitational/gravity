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

import cfg from 'app/config'
import Https from './../components/https/main'
import { fetchTlsCert } from './../flux/tls/actions';
import FeatureBase from './../../featureBase';
import * as featureFlags from './../../featureFlags';
import htmlUtils from 'app/lib/htmlUtils';
import { addNavItem } from './../flux/actions';
import { NavGroupEnum } from './../enums';

class CertFeature extends FeatureBase {

  constructor(routes) {        
    super();
    const route = {
      path: cfg.routes.settingsTlsCert,
      component: super.withMe(Https)
    };

    routes.push(route);        
  }
  
  getIndexRoute(){
    return cfg.routes.settingsTlsCert;
  }

  onload(context) {        
    const { siteId } = context;                
    const allowed = featureFlags.settingsCertificate();    
    const navItem = {
      icon: 'fa fa-certificate',
      title: 'HTTPS Certificate',
      to: htmlUtils.joinPaths(context.baseUrl, cfg.routes.settingsTlsCert)
    }

    if (!allowed) {
      this.handleAccesDenied();      
      return;
    }

    addNavItem(NavGroupEnum.SETTINGS, navItem);
    this.startProcessing();    
    return fetchTlsCert(siteId)
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this));    
  }  
}

export default CertFeature;