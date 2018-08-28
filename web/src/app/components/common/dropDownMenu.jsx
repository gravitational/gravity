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

const onClickMenu = e => { 
  e.stopPropagation(); 
  e.preventDefault(); 
}

export const Menu = props => (
  <div className={classnames("grv-dropdown-menu", props.className)} onClick={onClickMenu}>        
    <a className="dropdown-toggle" data-toggle="dropdown" href="#" role="button">
      {props.text} <span className="caret"></span>
    </a>          
    <ul className="dropdown-menu multi-level">       
      {props.children}
    </ul>
  </div>
)

export const SubMenu = props => (  
  <li className="dropdown-submenu">
    <a tabIndex="-1" href="#">                            
      <span>{props.text}</span>              
    </a>              
    <ul className="dropdown-menu">                             
      {props.children}          
    </ul>
  </li>           
)

export const MenuItem = props => (
  <li>{props.children}</li>
)

export const MenuItemDelete = props => (
  <li>
    <a title="Delete..." onClick={props.onClick}>
      <i className="fa fa-trash m-r-sm"/>
      <span>Delete...</span>
    </a>
  </li>
)

export const MenuItemDivider = () => (
  <li className="divider"/>
)

export const MenuItemTitle = props => (
  <div style={{fontSize: "11px"}} className="m-t-sm p-w-xs m-b-xs text-muted">              
    <span>{props.text}</span>              
  </div>                        
)

export const MenuItemLoginInput = props => (  
  <li className="grv-dropdown-menu-item-login-input">            
    <div className="input-group-sm m-b-xs">                          
      <i className="fa fa-terminal m-r"> </i>
      <input className="form-control" placeholder="Enter login name..." autoFocus {...props} />
    </div>  
  </li>     
)

export const MenuItemLogin = props => (
  <li>
    <a onClick={props.onClick}>
      <i className="fa fa-user-circle m-r-sm p-l-sm"/> {props.text}
    </a>
  </li>
)
