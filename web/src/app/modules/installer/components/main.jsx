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
import connect from 'app/lib/connect';
import {Failed} from 'app/components/msgPage';
import Indicator from 'app/components/common/indicator';
import * as actions from './../flux/installer/actions';
import getters from './../flux/installer/getters';

import StepIndicator from './stepIndicator';
import NewApp from './newApp/main.jsx';
import Provision from './provision/main.jsx';
import Progress from './progress/main.jsx';
import License from './license/main.jsx';
import {AppLogo} from './items';
import Eula from './eula.jsx';
import UserHints from './userHints';
import { StepValueEnum } from './../flux/enums';
import OverlayHost from 'app/components/common/overlayHost';


class Installer extends React.Component {

  renderStepComponent(){
    const { step, cfg } = this.props.model;
    switch (step) {
      case StepValueEnum.LICENSE:
        return <License {...cfg} />
      case StepValueEnum.NEW_APP:
        return <NewApp />
      case StepValueEnum.PROVISION:
        return <Provision />
      default:
        return <Progress />
    }
  }

  render() {
    const { model, initAttempt, logoUri } = this.props;
    const { step, displayName, stepOptions, cfg, eulaAccepted, eula } = model;
    const { isFailed, isProcessing, message } = initAttempt;

    if(isFailed){
      return <Failed message={message} />;
    }

    if(isProcessing){
      return <div><Indicator enabled={true} type={'bounce'}/></div>;
    }

    if(eula.enabled && !eulaAccepted){
      return (
        <Eula {...cfg}
          logoUri={logoUri}
          appName={displayName}
          onAccept={actions.acceptEula}
          content={eula.content}
          />
      )
    }

    return (
      <div className="grv-installer">
        <OverlayHost>
          <div style={boxContentStyle}>
            <div style={{ flex: "1", minWidth: "720px" }}>
              <div className="grv-installer-header">
                <div>
                  <AppLogo logoUri={logoUri}/>
                </div>
                <StepIndicator value={step} options={stepOptions}/>
              </div>
              <div className="grv-installer-content m-t-xl border-right">
                {this.renderStepComponent(step)}
              </div>
            </div>
            <div style={hintStyle}>
              <UserHints {...cfg} step={step}/>
            </div>
          </div>
        </OverlayHost>
      </div>
    );
  }
}

const boxContentStyle = {
  display: "flex",
  maxWidth: "1024px",
  margin: "0 auto"
}

const hintStyle = {
  maxWidth: "300px",
  paddingLeft: "25px"
}

function mapStateToProps() {
  return {
    model: getters.installer,
    logoUri: getters.logoUri,
    initAttempt: getters.initInstallerAttempt
  }
}

export default connect(mapStateToProps)(Installer);