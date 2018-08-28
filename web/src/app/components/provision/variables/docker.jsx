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
import ReactDOM from 'react-dom';
import { isObject } from 'lodash';
import $ from 'jQuery';
import classnames from 'classnames';
import WhatIs from 'app/components/whatis';
import { DISK_OPTION_AUTOMATIC } from 'app/flux/opAgent/constants';
import cfg from 'app/config';
import OverlayHost from 'app/components/common/overlayHost';

const AUTOMATIC_TEXT = 'loopback';
const AUTOMATIC_TEXT_TOOLTIP = 'not for production use';

const DockerVariable = props => {    
  let {
    value,
    label,
    options,
    onChange } = props;  
    
  let dockerCfg = cfg.getAgentDeviceDocker();
  let popoverProps = {
    title: dockerCfg.labelText,
    tooltip: dockerCfg.tooltipText,
    placement: 'top'
  }
        
  return (        
    <OverlayHost>
      <div className="form-group">
        <label>
          <span> Docker device </span>
          <WhatIs.ServerDevice {...popoverProps} />
        </label>
        <DiskInputDropDown
          value={value}
          classRules="required"
          onChange={onChange}
          options={options} />
      </div>    
    </OverlayHost>  
  )
}
    
const DiskInputDropDown = React.createClass({

  onClick(event){
    let {options} = this.props;
    let index = $(event.target).parents('li').index();
    let option = options[index];
    let value = isObject(option) ? option.value : option;
    this.props.onChange(value);

    // set focus on input box
    if(value !== DISK_OPTION_AUTOMATIC){
      ReactDOM.findDOMNode(this.refs.valueInput).focus();
    }
  },

  onValueChange(event){
    if( this.props.onChange){
      this.props.onChange(event.target.value);
    }
  },

  renderOption(option, index){
    let {label} = option;
    let isAutomatic = label === DISK_OPTION_AUTOMATIC;
    let [name, type, size ] = label.split('|');

    let displayValue = name;
    let description = `${type}: ${size}`;

    let iconClass = classnames('fa m-r-xs', {
      'fa-hdd-o' : !isAutomatic,
      'fa-exclamation-triangle text-warning' : isAutomatic
    });

    if(isAutomatic){
      description = AUTOMATIC_TEXT_TOOLTIP;
      displayValue = AUTOMATIC_TEXT;
    }

    return (
      <li  onClick={this.onClick} key={index}>
        <a href="#">
          <div style={{lineHeight:"normal"}}>
          <i className={iconClass}></i>
          <strong href="#"> {displayValue} </strong><br/>
          <small>{description}</small>
          </div>
        </a>
      </li>
    )
  },

  render(){
    let { options, value, classRules } = this.props;
    let $options = options.map(this.renderOption);
    let warningStyle = {
      float: 'right',
      marginRight: '6px',
      marginTop: '-22px',
      position: 'relative',
      zIndex: '2'
    }

    let isAutomatic = value === DISK_OPTION_AUTOMATIC;

    let iconClass = classnames('fa fa-exclamation-triangle text-warning', {
      'hidden': !isAutomatic
    });

    let displayValue = isAutomatic ? '' : value;

    return (
      <div className="grv-dropdown">
        <div className="input-group" >
          {
            options.length > 0 ?
              <div className="input-group-btn">
                <button type="button" className="btn btn-default dropdown-toggle" data-toggle="dropdown" aria-haspopup="trufe" aria-expanded="false">
                  <span className="caret"></span>
                </button>
                <ul className="dropdown-menu dropdown-menu-right">
                  {$options}
                </ul>
              </div> : null
          }

          <input ref="valueInput" placeholder={AUTOMATIC_TEXT} onChange={this.onValueChange} type="text" value={displayValue} className="form-control"/>
          <span style={warningStyle} title={AUTOMATIC_TEXT_TOOLTIP}className={iconClass}></span>
        </div>
        <input className={classRules} value={value} type="hidden"/>
      </div>
    )
  }
});

export default DockerVariable;