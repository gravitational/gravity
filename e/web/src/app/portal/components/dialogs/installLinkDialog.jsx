/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import reactor from 'oss-app/reactor';
import Button from 'oss-app/components/common/button';
import Indicator from 'oss-app/components/common/indicator';
import htmlUtils from 'oss-app/lib/htmlUtils';
import {
  GrvDialogFooter,
  GrvDialogContent,
  GrvDialogHeader,
  GrvDialog } from 'oss-app/components/dialogs/dialog';

import * as portalActions from './../../flux/apps/actions';
import portalGetters from './../../flux/apps/getters';

const errorIndicatorStyle = {
  'zIndex': '1',
  'flex': '1',
  'justifyContent': 'center',
  'display': 'flex',
  'alignItems': 'center'
}

const ErrorIndicator = () => (
  <div style={errorIndicatorStyle}>
    <i className="fa fa-exclamation-triangle fa-3x text-danger"></i>
    <div>
      <strong className="m-l">Failed to generate one time install link</strong>
    </div>
  </div>
)

const InstallLinkDialog = React.createClass({

  onCopyClick(link, event){
    event.preventDefault();
    htmlUtils.copyToClipboard(link);
    htmlUtils.selectElementContent(this.refs.link);
  },

  render(){
    let { onClose, message } = this.props;
    let { isProcessing, isFailed } = this.props;

    let hasLink = !isFailed && !isProcessing;

    return (
      <GrvDialog title="" className="grv-portal-dlg-install-link">
        <GrvDialogHeader>
          <div className="grv-portal-dlg-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className="fa fa-external-link fa-2x text-info" aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">One time install link</h3>
              <div>
                <small>
                  This link will allow users to start installation without logging into Ops Center.
                </small>
              </div>
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogContent>
          { isProcessing && <Indicator delay="none"/> }
          { isFailed && <ErrorIndicator/> }
          { hasLink &&
            <span ref="link" className="form-conrol grv-portal-dlg-install-link-value">
              {message}
            </span>
          }
        </GrvDialogContent>
        <GrvDialogFooter>
          { hasLink &&
            <button
              autoFocus
              onClick={this.onCopyClick.bind(this, message)}
              className="btn btn-primary m-r-xs">Click to copy</button>
          }
          <Button
            onClick={onClose}
            isPrimary={false}
            className="btn btn-white">
            Close
          </Button>
        </GrvDialogFooter>
      </GrvDialog>
    )
  }
})

const InstallLinkDialogContainer = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      createLinkAttemp: portalGetters.createInstallLinkAttemp
    }
  },

  getInitialState(){
    return { appId: null };
  },

  open(appId){
    portalActions.createOneTimeInstallLink(appId);
    this.setState({appId});
  },

  onClose(){
    this.setState({appId: null});
  },

  render(){
    let { appId, createLinkAttemp } = this.state;
    let props = {
      ...createLinkAttemp,
      ...this.props
    }

    if(!appId){
      return null;
    }else{
      return ( <InstallLinkDialog { ...props } onClose={this.onClose}/> );
    }
  }

});

export default InstallLinkDialogContainer;
