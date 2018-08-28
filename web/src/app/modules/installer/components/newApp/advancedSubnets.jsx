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
import WhatIs from 'app/components/whatis';
import Form from 'app/components/common/form';
import { connect } from 'nuclear-js-react-addons';
import getters from './../../flux/newApp/getters';
import * as actions from './../../flux/newApp/actions';
import { parseCidr } from 'app/lib/paramUtils';

const POD_HOST_NUM = 65534;
const VALIDATION_SUBNET = 'Invalid CIDR format';
const VALIDATION_POD_SUBNET_MIN = `Range cannot be less than ${POD_HOST_NUM}`;

class Subnets extends React.Component {
              
  onPodChange = e => actions.setOnpremSubnets(this.props.provider.serviceSubnet, e.target.value)
  onServiceChange = e => actions.setOnpremSubnets(e.target.value, this.props.provider.podSubnet)

  componentDidMount() {    
    $.validator.addMethod("grvSubnet", function (value) {      
      let result = parseCidr(value);
      return result !== null;
    }, VALIDATION_SUBNET);

    $.validator.addMethod("grvPodHostMin", function (value) {      
      let result = parseCidr(value);
      // ignore if null since grvSubnet method handles it
      if (result) {        
        return result.totalHost >= POD_HOST_NUM;
      }
      
      return true;
    }, VALIDATION_POD_SUBNET_MIN);

    const checks = {      
      'required': true,
      'grvSubnet': true      
    }

    $(this.refForm).validate({
      rules: {
        serviceSubnet: checks,
        podSubnet: {
          ...checks,
          grvPodHostMin: true
        }
      }
    })    
  }
  
  render() {    
    let { serviceSubnet, podSubnet } = this.props.provider;
    return (                      
      <Form refCb={e => this.refForm = e}>
        <div className="row">
          <div className="form-group col-sm-6">                          
            <label>
              <span>Service Subnet </span>
              <WhatIs.ServiceSubnet placement="top"/>
            </label>
            <input
              value={serviceSubnet}  
              className="form-control"
              name="serviceSubnet"
              placeholder="10.0.0.0/16"
              onChange={this.onServiceChange} />    
        </div>
        <div className="form-group  col-sm-6">              
          <label>
            <span>Pod Subnet </span>
            <WhatIs.PodSubnet placement="top"/>
          </label>  
          <input ref="secretKey"                
            value={podSubnet}
            className="form-control"
            name="podSubnet"
            placeholder="10.0.0.0/16"
            onChange={this.onPodChange} />
          </div>
        </div>
      </Form>            
    )
  }    
}

function mapStateToProps() {
  return {    
    provider: getters.onpremProvider    
  }
}

export default connect(mapStateToProps)(Subnets);