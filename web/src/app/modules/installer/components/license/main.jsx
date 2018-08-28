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
import React from 'react';
import reactor from 'app/reactor';
import getters from './../../flux/newApp/getters';
import * as actions from './../../flux/newApp/actions';

import LicenseField from './licenseField';
import Footer from './../footer.jsx';

const AppLicense = React.createClass({

  mixins: [reactor.ReactMixin],

  propTypes: {
   licenseHeaderText: React.PropTypes.string.isRequired,
   licenseOptionText: React.PropTypes.string.isRequired,
   licenseOptionTrialText: React.PropTypes.string.isRequired
  },

  getDataBindings() {
    return {
      newApp: getters.newApp,
      verifyLicenseAttemp: getters.verifyLicenseAttemp
    }
  },
  
  onContinue(){    
    let { packageName } = this.state.newApp;    
    if($(this.refs.form).valid()){
      let license = this.refs.licenseField.getValue();
      actions.setDeploymentType(license, packageName);
    }    
  },

  render() {
    let { verifyLicenseAttemp } = this.state;
    let { licenseHeaderText } = this.props;
    return (
      <div ref="container">
        <div className="m-t">
          <h2>{licenseHeaderText}</h2>          
          <form ref="form" className="m-l m-t-lg">
            <LicenseField ref="licenseField"/>
          </form>          
        </div>
        <Footer 
          text="Continue" 
          attemp={verifyLicenseAttemp} 
          onClick={this.onContinue} />
      </div>
    );
  }
});

export default AppLicense;