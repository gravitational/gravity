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
import { capitalize } from 'lodash';
import cfg from 'app/config';
import WhatIs from 'app/components/whatis';
import OverlayHost from 'app/components/common/overlayHost';

let key = 0;

const MountVariable = React.createClass({

  getInitialState(){
    return {
      key: key++
    }
  },

  onValueChange(event){
    if(this.props.onChange){
      this.props.onChange(event.target.value);
    }
  },

  render(){
    let { value, name } = this.props;
    let key = 'mnt-' + this.state.key;
    
    let mountCfg = cfg.getAgentDeviceMount(name);
    let title = mountCfg.labelText || name;
    title = capitalize(title);

    let $popover = null;

    // display 'what is it' icon with popover
    if (mountCfg.tooltipText) {
      let popoverProps = {
        title,
        tooltip: mountCfg.tooltipText,
        placement: 'top'
      } 
      
      $popover = (
        <WhatIs.ServerDevice {...popoverProps} />          
      )
    }    

    return (      
      <OverlayHost>
        <div className="form-group">        
          <label>
            <span> {title} </span>
            {$popover}
          </label>
          <div className="input-group" >
            <input ref="valueInput"
              name={key}
              onChange={this.onValueChange}
              type="text"
              value={value}
              className="form-control required"/>
          </div>
        </div>
      </OverlayHost>  
    )
  }
});

export default MountVariable;