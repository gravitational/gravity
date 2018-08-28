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
import NavLeftBar, { NavGroup } from './nav/nav';
import settingsGetters from './../flux/getters';
import NavTopBar from './navTopBar';
import Indicator from 'app/components/common/indicator';
import { Failed } from 'app/components/msgPage.jsx';
import { NavGroupEnum } from './../enums';
import connect from 'app/lib/connect'

class Settings extends React.Component {

  render() {
    const { settings, attempt } = this.props;
    const { baseLabel, goBackUrl } = settings;
    const { isFailed, isProcessing, isSuccess, message } = attempt;

    if(isFailed){
      return <Failed message={message} />;
    }

    if(isProcessing){
      return <div><Indicator enabled={true} type={'bounce'}/></div>;
    }

    if (!isSuccess) {
      return null;
    }

    return (
      <div className="grv-settings">
        <NavTopBar
          customerLogoUri={settings.getLogoUri()}
          goBackUrl={goBackUrl}
          goBackLabel={baseLabel} />
        <div className="grv-settings-content m-t m-b">
          <NavLeftBar className="m-l-sm m-r">
            <NavGroup title={NavGroupEnum.USER_GROUPS} items={settings.getNavGroup(NavGroupEnum.USER_GROUPS)}/>
            <NavGroup title={NavGroupEnum.APPLICATION} items={settings.getNavGroup(NavGroupEnum.APPLICATION)}/>
            <NavGroup title={NavGroupEnum.SETTINGS} items={settings.getNavGroup(NavGroupEnum.SETTINGS)}/>
          </NavLeftBar>
          {this.props.children}
        </div>
      </div>
    )
  }
}

const mapFluxToProps = () => {
  return {
    settings: settingsGetters.settings,
    attempt: settingsGetters.initSettingsAttempt
  }
}

export default connect(mapFluxToProps)(Settings);