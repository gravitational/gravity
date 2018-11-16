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
import React, { PropTypes } from 'react';
import connect from 'app/lib/connect';
import getters from './../../flux/newApp/getters';
import * as actions from './../../flux/newApp/actions';
import Footer from './../footer.jsx';
class AppLicense extends React.Component {

  static propTypes = {
   licenseHeaderText: PropTypes.string.isRequired,
   licenseOptionText: PropTypes.string.isRequired,
   licenseOptionTrialText: PropTypes.string.isRequired
  }

  onContinue = () => {
    const { packageName } = this.props.newApp;
    if($(this.refs.form).valid()){
      const license = this.refLicense.value;
      actions.setDeploymentType(license, packageName);
    }
  }

  render() {
    const { licenseHeaderText, attempt } = this.props;
    return (
      <div ref="container">
        <div className="m-t">
          <h2>{licenseHeaderText}</h2>
          <form ref="form" className="m-l m-t-lg">
            <div className="grv-installer-license">
              <div className="form-group">
                <label><i className="fa fa-key"></i> License</label>
                <textArea ref={ e => this.refLicense = e }
                  className="form-control required grv-license" autoComplete="off" type="text"
                  autoFocus
                  required
                  placeholder="Insert your license key here"/>
              </div>
            </div>
          </form>
        </div>
        <Footer
          text="Continue"
          attemp={attempt}
          onClick={this.onContinue} />
      </div>
    );
  }
}

function mapStateToProps() {
  return {
    newApp: getters.newApp,
    attempt: getters.verifyLicenseAttempt
  }
}

export default connect(mapStateToProps)(AppLicense);