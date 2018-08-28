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
import { Separator } from './../items';
import { makeDownloadable } from 'app/services/downloader';

export const Success = ({siteUrl}) => (
  <div className="grv-installer-progress-result">
    <i style={{'fontSize': '100px'}} className="fa fa-check" aria-hidden="true"/>
    <h3 className="m-t-lg">Installation Successful</h3>
    <h3>Click "Continue" to finish configuring the application.</h3>
    <a className="btn btn-primary m-t" href={siteUrl}>Continue</a>
    <div className="m-t-xl m-b-lg">
      <Separator/>
    </div>
  </div>
)
  
export class Failure extends React.Component {  

  onClick = () => {    
    location.href= makeDownloadable(this.props.tarballUrl);
  }

  render(){
    return (
      <div className="grv-installer-progress-result">
        <i style={{'fontSize': '100px'}} className="fa text-danger fa-exclamation-triangle" aria-hidden="true"></i>
        <h2 className="m-t-lg">Install failure</h2>
        <p className="m-t-lg">Something went wrong with the install. We've attached a tarball which has diagnostic logs that our team will need to review. We sincerely apologize for any inconvenience.</p>
        <button onClick={this.onClick} className="btn m-t-lg m-b btn-primary center-block">
          <i className="fa fa-cloud-download" aria-hidden="true">  </i>
          <span className="m-l-xs">Download tarball</span>
        </button>
        <div className="m-t-xl m-b-lg">
          <Separator/>
        </div>
    </div>
    )
  }
}