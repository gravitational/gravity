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
import {Separator, ToolBar} from './items';
import htmlUtils from 'app/lib/htmlUtils';
import * as actions from './../../flux/currentSite/actions';
import { forEach } from 'lodash';
import Button from 'app/components/common/button';

var LicenseStatus = ({isActive, isError, isExpired, message=''}) => {
  let text = 'undefined';
  let iconClass = classnames('label m-l m-r', {
    'label-primary': isActive,
    'label-danger': isError,
    'label-warning': isExpired
  });

  text = isActive ? 'Active' : text;
  text = isError ? 'Error' : text;
  text = isExpired ? 'Expired' : text;

  return (
    <span>
      <span className={iconClass}>{text}</span>
      <small>{message}</small>
    </span>
 ) ;
}

var AppLicense = React.createClass({

  propTypes: {
    license: React.PropTypes.object.isRequired,
    updateAttemp: React.PropTypes.object.isRequired
  },

  onUpdate(){
    actions.updateLicense(this.refs.license.value);
  },

  onDownloadLicense(){
    let { raw } = this.props.license;
    htmlUtils.download('license.txt', raw );
  },

  componentWillReceiveProps(nextProps){
    let { isSuccess } = nextProps.updateAttemp;
    // reset the element value in case of successfull attempt
    if(!this.props.updateAttemp.isSuccess && isSuccess){
      this.refs.license.value = '';
    }
  },

  render(){
    if(!this.props.license){
      return null;
    }

    let { updateAttemp } = this.props;
    let { info = {}, status } = this.props.license
    let $infoItems = [];

    forEach(info, (value, title) => {
      $infoItems.push(<dt className="col-sm-2" key={'dt' + $infoItems.length}>{title}</dt>);
      $infoItems.push(<dd className="col-sm-9" key={'dd' + $infoItems.length}>{value}</dd>);
    });

    return (
      <div className="grv-site-app-license">
        <ToolBar>
          <h3 className="grv-site-app-h3">
            <i className="fa fa-key fa-lg m-r" aria-hidden="true"></i>
            <span>License <LicenseStatus {...status}/> </span>
          </h3>
          <div>
            <a href="#" className="btn btn-primary btn-sm" onClick={this.onDownloadLicense}>
              <i className="fa fa-cloud-download"></i>  Download
            </a>
          </div>
        </ToolBar>
        <Separator/>
        <div className="m-t-sm row">
          <div className="col-sm-12">
            <h4>Details</h4>
            <dl className="">
              {$infoItems}
            </dl>
          </div>
        </div>
        <div className="row m-t-md">
          <div className="col-sm-12">
            <textArea ref="license" className="form-control grv-license" autoComplete="off" type="text" aria-required="true" aria-invalid="false"
              placeholder="Insert new license here"/>
            <div className="pull-right2 m-t-sm">
              <Button {...updateAttemp} isPrimary={false} className="btn-primary btn-sm" onClick={this.onUpdate}>
                Update license
              </Button>
            </div>
          </div>
        </div>
      </div>
    )
  }
});

export default AppLicense;