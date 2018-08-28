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
import * as Messages from 'app/components/msgPage.jsx';
import settingsGetters from './../flux/getters';
import connect from 'app/lib/connect'

class SettingsIndex extends React.Component {      
  
  static propTypes = {
    router: React.PropTypes.object.isRequired,
    location: React.PropTypes.object.isRequired,
    settingsStore: React.PropTypes.object.isRequired
  }

  componentDidMount(){        
    const route = this.getAvailableRoute();
    if(route){      
      this.props.router.replace({ pathname: route })
    }
  }

  getAvailableRoute(){
    let navItem = null;
    const { settingsStore } = this.props;    
    const first = settingsStore.navGroups.first();    
    if (first){
      navItem = first.get(0)
    }

    return navItem ? navItem.to : ''
  }

  render(){
    return ( <Messages.AccessDenied/> )
  }
}

const mapFluxToProps = () => {
  return {      
    settingsStore: settingsGetters.settings    
  }
}

export default connect(mapFluxToProps)(SettingsIndex);