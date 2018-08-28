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
import { Link } from 'react-router'
import classnames from 'classnames';

export class NavGroup extends React.Component {    

  static contextTypes = {
    router: React.PropTypes.object.isRequired
  }
  
  render() {          
    const { items = [], title } = this.props;

    if (items.length === 0) {
      return null;
    }

    const $items = items.map(( i, index) => {          
      const className = classnames('grv-settings-nav-group-menu-item', {
        'active': this.context.router.isActive(i.to, i.isIndex)
      } ) 

      return (
        <li key={index} className={className}>
          <Link to={i.to} >
            <i className={i.icon}/>
            <span className="nav-label">
              {i.title}
            </span>
          </Link>
        </li>
      );
    });
    
    return (
      <nav className="grv-settings-nav-group" role="navigation">        
        <ul className="grv-settings-nav-group-menu">
          <li className="grv-settings-nav-group-header">
            <h4 className="no-margins">{title}</h4>
          </li>
          {$items}
        </ul>        
      </nav>
    );
  }
}
  
const Nav = props =>  (                                 
  <div className={classnames("grv-settings-nav-groups", props.className)}>      
    {props.children}        
  </div>        
)
  
export default Nav;