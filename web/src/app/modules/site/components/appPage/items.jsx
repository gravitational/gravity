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

export const Separator = () => (<div className="grv-line-solid m-t-lg m-b-lg"></div>);

export const VersionLabel = ({version})=> (
  <span className="m-l label label-primary grv-site-app-label-version">Version {version}</span>
)

export const Error = React.createClass({
  render(){
    return(
      <div className="row">
        <h2 className="col-sm-12 text-danger">
          <i className="fa fa-exclamation-triangle fa-lg m-r" aria-hidden="true"></i>
          Issues need to be resolved
        </h2>
        <div className="col-sm-12 m-t">
          <p className="text-danger"> There was a failed attemp to provision the neccessary HDD space. Please check your logs and resolve.</p>
        </div>
      </div>
    )
  }
})

export const ToolBar = (props) => (
  <div style={ToolBar.style}>
    {props.children}
  </div>
)

ToolBar.style = {
  display: 'flex',
  justifyContent: 'space-between'
}