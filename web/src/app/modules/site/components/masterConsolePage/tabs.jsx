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
export class TabItem extends React.Component {

  static childContextTypes = {
    isTabActive: React.PropTypes.bool
  }

  getChildContext() {
    let {isActive} = this.props;
    return {
      isTabActive: isActive
    };
  }

  render(){
    let {isActive, children} = this.props;
    let tabClass = classnames('grv-site-console-tab-pane', {
      'hidden': !isActive
    })

    return (
      <div className={tabClass}>
        {children}
      </div>
    )
  }
}

export class ServerTabs extends React.Component {

  onHeaderClick(item){
    const index = this.props.children.indexOf(item);
    const { onTabClick } = this.props
    if(onTabClick){
      onTabClick(index);
    }
  }

  onTabClose(item){
    const index = this.props.children.indexOf(item);
    const { onTabClose } = this.props
    if(onTabClose){
      onTabClose(index);
    }
  }

  renderHeader(item, key){
    const { value } = this.props;
    const { title, canClose = true } = item.props;
    const closeBtnClass = classnames('grv-site-console-tabs-close', {
      'hidden': !canClose
    })

    const headerClass = classnames('grv-site-console-tabs-header',{
      '--active': key === value
    })

    return (
      <li key={key} className={headerClass}>
        <div className="grv-site-console-tabs-header-title" onClick={()=> this.onHeaderClick(item)}>
          {title}
        </div>
        <div className={closeBtnClass} onClick={ () => this.onTabClose(item) }>
          <i className="fa fa-times-circle" aria-hidden="true"></i>
        </div>
      </li>
    )
  }

  renderTabPane(item, key){
    let $content = null;
    const isActive = key === this.props.value;
    if (React.isValidElement(item)) {
       $content = React.cloneElement(item, {isActive, ...item.props});
     }

    return $content;
  }

  render() {
    const children = [];
    React.Children.forEach(this.props.children, (child) => {
      children.push(child);
    });

    const $headers = children.map(this.renderHeader.bind(this));
    const $tabPanes = children.map(this.renderTabPane.bind(this));

    return (
      <div className="grv-site-console-tabs-container">
        <ul className="grv-site-console-tabs-headers">
          {$headers}
        </ul>
        <div className="grv-site-console-tab-content">
          {$tabPanes}
        </div>
      </div>
    );
  }

}