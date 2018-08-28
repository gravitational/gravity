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
import AppTags from './advancedAppTags';
import SubnetCidr from './advancedSubnets';
import { ProviderEnum } from 'app/services/enums';

class AdvancedOptions extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      isExpanded: false
    }
  }

  onToggleExpand = () => {
    let {isExpanded} = this.state;
    this.setState({
      isExpanded: !isExpanded
    });
  }

  render() {
    let { isExpanded } = this.state;
    let { tags, subnets, onChangeTags, provider } = this.props;    
    let shouldDisplaySubnets = provider === ProviderEnum.ONPREM;
    let $content = null;
    let iconClass = classnames('fa', {
      'fa-chevron-up': isExpanded,
      'fa-chevron-down': !isExpanded
    })

    let containerClass = classnames('grv-installer-advanced', {
      '--expanded': isExpanded
    })
    
    if (isExpanded) {
      $content = (
        <div className="grv-installer-advanced-content">
          {shouldDisplaySubnets && <SubnetCidr subnets={subnets} />}
          <AppTags tags={tags} onChange={onChangeTags} />            
        </div>
      )              
    }
    
    return (
      <div className={containerClass}>
        <div onClick={this.onToggleExpand} className="grv-installer-advanced-header">
          <a className="grv-installer-aws-btn-expander m-r-xs">
            <i className={iconClass}></i>
          </a>
          <label className="">Advanced Options</label>
        </div>
        {$content}        
      </div>
    );
  }
}

export default AdvancedOptions;
