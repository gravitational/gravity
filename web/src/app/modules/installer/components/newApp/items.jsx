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
import classnames from 'classnames';
import RadioGroup from 'app/components/common/radioGroup.jsx';
import { ProviderEnum } from 'app/services/enums';

class NewOrExistingServers extends React.Component {

  onChange = value => {
    this.props.onChange(value === 'existing');
  }

  render() {
    const { useExisting } = this.props;
    const value = useExisting ? 'existing' : 'newServer';
    const options = [
      {
        value: 'newServer',
        title: (
          <span>
            <span> Provision new servers </span>
          </span>
        )
      },
      {
        value: 'existing',
        title: (
          <span>
            <span> Use existing servers </span>
          </span>
        )
      }
    ]

    return (
      <div className="form-group m-t-xs">
        <RadioGroup options={options} value={value} onChange={this.onChange}/>
      </div>
    );
  }
}

class ProviderOptions extends React.Component {

  static propTypes = {
    options: React.PropTypes.array.isRequired,
    value: React.PropTypes.string
  }

  onChange = providerType => {
    this.props.onChange(providerType)
  }

  renderProviderIcon(providerType){
    let iconClass = classnames('grv-installer-icon', {
      '--aws': providerType === ProviderEnum.AWS,
      '--metal': providerType === ProviderEnum.ONPREM
    });

    return <div className={iconClass}/>
  }

  render(){
    const { options, value } = this.props;
    const $options = options.map((providerType, index) =>{

      const itemClass = classnames('grv-item', {
        'grv-active': value === providerType,
      });

      return(
        <div onClick={this.onChange.bind(this, providerType)} key={index} className={itemClass}>
          {this.renderProviderIcon(providerType)}
        </div>
      );
    });

    return (
      <div className="grv-installer-provider">
        <h3 className="m-t-xlg m-b">Choose provider</h3>
        <div className="grv-installer-provider-list">
          {$options}
        </div>
      </div>
    )
  }
}

export {
  NewOrExistingServers,
  ProviderOptions
}