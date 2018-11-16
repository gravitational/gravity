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
import $ from 'jQuery';
import {debounce} from 'lodash';
import Form from 'app/components/common/form';
import connect from 'app/lib/connect';
import {isDomainName} from 'app/lib/paramUtils';
import getters from './../../flux/newApp/getters';
import * as actions from './../../flux/newApp/actions';

const VALIDATION_ERROR_MSG = 'Not a valid name';

const  debouncedSetDomainName = debounce(actions.setDomainName, 400);

class DomainName extends React.Component {

  componentDidMount(){
    $.validator.addMethod("domain", function(value, element){
      return this.optional(element) || isDomainName(value);
    }, VALIDATION_ERROR_MSG);

    $(this.refForm).validate({

      errorPlacement: $error => {
        $(this.refFormErrors)
          .empty()
          .append($error);
      },

      unhighlight: (element, errorClass) => {
        $(this.refFormErrors).empty()
        $(element).removeClass(errorClass);
      },

      rules:{
        domainName:{
          required: true,
          domain: true
        }
      }
    })
  }

  onChange = event => {
    const $form = $(this.refForm);
    debouncedSetDomainName.cancel();
    if($form.valid()){
      debouncedSetDomainName(event.target.value)
    }else{
      actions.setDomainNameVerifiedFlag(false);
    }
  }

  render(){
    const { isProcessing, isFailed, message } = this.props.attempt;
    const { isDomainNameValid, domainName, name } = this.props.newApp;
    const hintText = `Please enter a unique deployment name. Example: "${name}.yourcompany"`;

    return(
      <div className="m-t-lg m-b-lg">
        <h3>Cluster Name</h3>
        <div className="grv-installer-fqdn">
          <Form className="input-group col-sm-12 col-xs-12" refCb={ e => this.refForm =e }>
            <span className="grv-installer-fqdn-indicator">
              { isProcessing && <i className="fa fa-cog fa-spin fa-lg"/> }
              { isDomainNameValid && <i className="fa fa-check fa-lg"/> }
            </span>
            <input className="form-control" name="domainName" autoComplete="off" type="text"
              autoFocus
              onChange={this.onChange}
              defaultValue={domainName}
              placeholder="Cluster name"/>
          </Form>
          <div className="grv-installer-fqdn-errors" ref={ e => this.refFormErrors = e } />
          {isFailed && (
            <div className="grv-installer-fqdn-errors">
              <label className="error" htmlFor="domainName">{message}</label>
            </div>
          )}
        </div>
        <div className="help-block">{hintText}</div>
    </div>
    )
  }
}

function mapStateToProps() {
  return {
    newApp: getters.newApp,
    attempt: getters.validateDomainNameAttempt
  }
}

export default connect(mapStateToProps)(DomainName);
