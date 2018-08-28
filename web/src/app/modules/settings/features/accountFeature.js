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
import htmlUtils from 'app/lib/htmlUtils';
import FeatureBase from '../../featureBase'
import Account from './../components/account/main';      
import * as featureFlags from './../../featureFlags';
import { addNavItem } from './../flux/actions';
import { NavGroupEnum } from './../enums';

class AccountFeature extends FeatureBase {  
  constructor(routes) {
    super()
    routes.push({       
      path: cfg.routes.settingsAccount, 
      component: super.withMe(Account) 
    })
  }

  getIndexRoute(){
    return cfg.routes.settingsAccount;
  }

  onload(context) {                      
    const allowed = featureFlags.settingsAccount();    
    const navItem = {
      icon: "fa fa-user",    
      title: "My Account",            
      to: htmlUtils.joinPaths(context.baseUrl, cfg.routes.settingsAccount)
    }

    if (allowed) {
      addNavItem(NavGroupEnum.USER_GROUPS, navItem);
    }else{
      this.handleAccesDenied();            
    }        
  }  
    
}

export default AccountFeature