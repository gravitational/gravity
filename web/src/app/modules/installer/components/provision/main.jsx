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
import Button from 'app/components/common/button';

import * as actions from './../../flux/provision/actions';
import getters from './../../flux/provision/getters';


import Profiles from './profiles';
import Footer, { Warning } from'./../footer';
import {FlavorSelector} from'./flavorSelector';

const Success = () => {      
  return (
    <div className="grv-installer-attemp-message">
      <i className="fa fa-check --success" aria-hidden="true"></i>      
    </div>
  );
};

class ProvisionFooter extends Footer {
  constructor(props, context) {
    super(props, context);    
    this.state = {
      isPreCheck: false  
    }  
  }    

  onContinue() {
    this.onClick(this.props.onClick);      
    this.setState({isPreCheck: false});
  }

  onPreCheck() {   
    this.onClick(this.props.onVerify);    
    this.setState({isPreCheck: true});
  }

  renderAttempMessage(attemp) {      
    let {isFailed, isSuccess, message} = attemp;        
    let $msg = null;  
    if (isSuccess) {
      $msg = <Success/>
    } else if (isFailed) {
      $msg = <Warning data={message}/>  
    }  
    return ( <div> {$msg} </div> )      
  }

  renderVerifyBtn(onClickFn, isDisabled, isProcessing) {            
    return (
      <Button className="btn btn-default m-r"
        key="footer-verify-btn"        
        isProcessing={isProcessing}
        onClick={() => this.onClick(onClickFn)}
        isDisabled={isDisabled} >            
        <span>Verify</span>
      </Button>
    )
  }

  renderBtnGroup() {      
    let { isPreCheck } = this.state;
    let { text, isOnPrem, onPremServerCount, attemp } = this.props;

    // use default footer btn group if not on prem      
    if (!isOnPrem) {
        return super.renderBtnGroup();  
    }

    // install btn states                
    let isStartBtnProcessing = attemp.isProcessing && !isPreCheck;
    let isStartBtnDisabled = (attemp.isProcessing && isPreCheck) || onPremServerCount === 0;
    // verify btn states
    let isVerifyBtnProcessing = attemp.isProcessing && isPreCheck;  
    let isVerifyBtnDisabled = (isStartBtnProcessing && !isPreCheck) || onPremServerCount === 0;    
    

    if (onPremServerCount === 0) {
      text = "Waiting for servers..."
    }

    let $primaryBtn = this.renderPrimaryBtn(
        text,
        this.onContinue.bind(this),
        isStartBtnProcessing,
        isStartBtnDisabled
      );
      
    let $verifyBtn = this.renderVerifyBtn(
        this.onPreCheck.bind(this),
        isVerifyBtnDisabled,
        isVerifyBtnProcessing        
    );
      
    return super.renderBtnGroup([$verifyBtn, $primaryBtn]);
  }
}

const Provision = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      onPremServerCount: getters.onPremServerCount(),
      model: getters.provision,
      startInstallAttemp: getters.startInstallAttempt      
    }
  },

  render() {
    let { model, onPremServerCount, startInstallAttemp } = this.state;
    let { flavorsSelector, flavorsTitle, isOnPrem } = model;
    let shouldDisplayflavorSelector = flavorsSelector && flavorsSelector.options.length > 1;
    return (
      <div>
        { shouldDisplayflavorSelector  ?
          <div className="m-b-xl">
            <h2>{flavorsTitle}</h2>
            <FlavorSelector {...flavorsSelector} onChange={actions.setFlavorNumber}/>
          </div> :
          <div className="m-b-lg">
            <h2>Review Infrastructure Requirements</h2>
          </div>
        }
        <Profiles {...model} />                                
        <ProvisionFooter
          text="Start Installation"
          onClick={actions.startInstall}
          onVerify={actions.startInstallPrecheck}
          attemp={startInstallAttemp}            
          isOnPrem={isOnPrem}     
          onPremServerCount={onPremServerCount}
        />
      </div>
    );
  }
});

export default Provision;
