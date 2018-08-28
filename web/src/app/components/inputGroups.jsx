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
import WhatIs from './whatis';
import classnames from 'classnames';

export const AwsAccessKey = ({ hintPlacement="top", className, onChange}) => (  
  <div className={ classnames("form-group", className) }>
    <label>
      <span> Access key </span>
      <WhatIs.AwsAccessKey placement={hintPlacement}/>
    </label>
    <input autoFocus autoComplete="off" type="text"      
      className="form-control required"
      name="aws_access_key"
      placeholder="AKIAIOSFODNN7EXAMPLE"            
      onChange={e => onChange(e.target.value) }
      />
  </div>
)

export const AwsSecretKey = ({ hintPlacement="top", className, onChange}) => (  
  <div className={ classnames("form-group", className) }>
    <label>
      <span> Secret key </span>
      <WhatIs.AwsAccessKey placement={hintPlacement}/>
    </label>
    <input autoComplete="off" type="text"
      className="form-control required"
      name="aws_secret_key"
      placeholder="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" 
      onChange={ e => onChange(e.target.value) }
    />
  </div>
)

export class AwsSessionToken extends React.Component {

  constructor(props) {
    super(props)

    this.state = {
      isExpanded: false
    }
  }

  onToggle = () => {
    if (this.state.isExpanded) {
      this.props.onChange('');
    }

    this.setState({
      isExpanded: !this.state.isExpanded
    })    
  }
  
  render() {
    let { onChange, className } = this.props;
    let { isExpanded } = this.state;
    return (      
      <div className={ classnames("form-group", className) }>
        <div className="checkbox no-margins">
          <label>
            <input type="checkbox" style={{ marginTop: "3px" }} checked={isExpanded} onChange={this.onToggle} />
            <span> Use session token </span>
            <WhatIs.AwsSessionToken placement="top"/>
          </label>
        </div>      
        {
          isExpanded &&
          <div>
            <input autoComplete="off"
              className="form-control m-t-xs required"
              name="aws_session_token"
              placeholder="FQoDYXdzEHsaDGV2WyeFJbWM6vfdxpngd3VVIIyj0tj7qc9V/qRUVrc8QUdcoOKgkt649VrXP0dK/0X..." type="text"
              onChange={e => onChange(e.target.value)}
            />
          </div>
        }
        </div>                                                                                                                                                   
    )        
  }
}
