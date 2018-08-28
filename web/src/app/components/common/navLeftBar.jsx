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
import classNames from 'classnames';
import { Link } from 'react-router';
import { GravitationalLogo } from './../icons';
import cfg from 'app/config';

const DefaultHeader = ()=> (
  <span>
    <a href={cfg.routes.app}>
      <GravitationalLogo/>
    </a>
  </span>
)

class NavLeftBar extends React.Component {

  static contextTypes = {
    router: React.PropTypes.object.isRequired
  }

  hasActiveChildren(item) {    
    let children = item.children || [];
    let hasAny = children.some(i => {
      return this.context.router.isActive(i.to, i.isIndex);
    });

    return hasAny;        
  }

  renderItem(i, index) {             
    let className = classNames({
      'active': this.context.router.isActive(i.to, i.isIndex),
      'grv-nav-active-child': this.hasActiveChildren(i)
    })
    
    let children = i.children || [];    
    let $subMenu = null;        
    if (children.length > 0) {
      let $childrenItems = children.map(this.renderItem.bind(this));
      $subMenu = (
        <ul className="nav nav-second-level"> {$childrenItems} </ul>
      )
    }

    return (
      <li key={index} className={className}>
        <Link to={i.to} >
          <i className={i.icon}/>
          <span className="nav-label">
            {i.title}
          </span>
        </Link>
        {$subMenu}
      </li>
    );    
  }

  render(){
    let { menuItems=[], header } = this.props;    
    let items = menuItems.map(this.renderItem.bind(this));
    let $header = null;

    if (React.isValidElement(header)) {
      $header = React.cloneElement(header);
    }else{
      $header = <DefaultHeader/>;
    }

    return (
      <nav className="navbar-default grv-nav navbar-static-side" role="navigation">
        <div className="sidebar-collapse">
          <ul className="nav metismenu" id="side-menu">
            <li className="nav-header">
            {$header}
            </li>
            {items}
          </ul>
        </div>
      </nav>
    );
  }    
}

export default NavLeftBar;
