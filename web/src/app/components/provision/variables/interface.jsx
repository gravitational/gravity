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
import cfg from 'app/config';
import { DropDown } from 'app/components/common/dropDown';
import WhatIs from 'app/components/whatis';
import OverlayHost from 'app/components/common/overlayHost';

let id = 0;

const InterfaceVariable = React.createClass({

  propTypes: {
   options: React.PropTypes.array
  },

  getInitialState(){
    return {
      id: id++
    };
  },

  render() {    
    let { options, value, label, onChange } = this.props;
    
    let ipCfg = cfg.getAgentDeviceIpv4();
    let title = ipCfg.labelText || 'IP Address';

    let popoverProps = {
      title,
      tooltip: ipCfg.tooltipText,
      placement: 'top'
    }
    
    options = options.map((item) => ({
      value: item,
      label: item
    }));
        
    return (      
      <OverlayHost>
        <div className="form-group grv-provision-req-server-interface">        
          <label>
            <span> {title} </span>
            <WhatIs.IpAddress  {...popoverProps} />
          </label>
          <DropDown
            classRules="required"
            name={"ips-"+this.state.id}
            value={value}
            onChange={onChange}
            searchable={true}
            clearable={false}
            options={options}
          />
        </div>      
      </OverlayHost>  
    )
  }
})

export default InterfaceVariable;