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
import reactor from 'app/reactor';
import getters from './../flux/currentSite/getters';
import NavLeftBar from 'app/components/common/navLeftBar.jsx';
import { CustomerLogo } from 'app/components/icons';

var SiteLeftNav = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {    
    return {
      nav: getters.currentSiteNav
    }
  },

  getInitialState(){
    let { appInfo } = reactor.evaluate(getters.currentSite());    
    this.logoUri = appInfo.logoUri;
    return null;
  },

  render() {    
    let { nav } = this.state;
    let $header = null;
    if(this.logoUri){
      $header = <CustomerLogo className="grv-site-customer-logo" imageUri={this.logoUri} />
    }

    return <NavLeftBar header={$header} menuItems={nav} />
  }

});

export default SiteLeftNav;
