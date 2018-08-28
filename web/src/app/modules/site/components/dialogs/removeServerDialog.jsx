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

import $ from 'jQuery';
import React from 'react';
import Form from 'app/components/common/form';
import Button from 'app/components/common/button';
import { ProviderEnum } from 'app/services/enums';
import OverlayHost from 'app/components/common/overlayHost';
import * as InputGroups from 'app/components/inputGroups';
import {
  GrvDialogHeader,
  GrvDialogContent,
  GrvDialogFooter,
  GrvDialog } from  'app/components/dialogs/dialog';


const renderAwsHeaderText = hostname => (
  <small>
    You are about to delete instance <strong>{hostname}</strong>
    <br/>
    Deleting the instance will remove it from the cluster and deprovision it.
  </small>
)

const renderHeaderText = hostname => (
  <small>
    You are about to delete server <strong>{hostname}</strong>
    <br/>
    Deleting a server will remove application from it.
  </small>
)

class RemoveServerDialog extends React.Component {

  static propTypes = {
    onContinue: React.PropTypes.func.isRequired,
    onCancel: React.PropTypes.func.isRequired,
    attemp: React.PropTypes.object.isRequired,
    hostname: React.PropTypes.string.isRequired,
    provider: React.PropTypes.string.isRequired
  }

  constructor(props) {
    super(props)
    this.secretKey = '';
    this.accessKey = '';
    this.sessionToken = '';
  }

  onAccessKeyChange = value => {
    this.accessKey = value;
  }

  onSecretKeyChange = value => {
    this.secretKey = value;
  }

  onSessionKeyChange = value => {
    this.sessionToken = value;
  }

  onContinue = () => {
    if(this.isValid()){
      this.props.onContinue({
        hostname: this.props.hostname,
        secretKey: this.secretKey,
        accessKey: this.accessKey,
        sessionToken: this.sessionToken,
       });
    }
  }

  isValid() {
    const $form = $(this.refForm);
    return $form.length === 0 || $form.valid();
  }

  render() {
    const { hostname, provider, attemp, onCancel } = this.props;
    const isAws = provider === ProviderEnum.AWS;
    const dialogClass = 'grv-dialog-no-body grv-dialog-confirm';
    const { isProcessing } = attemp;
    const $headerText = isAws ? renderAwsHeaderText(hostname) : renderHeaderText(hostname);

    return (
      <GrvDialog ref="dialog" className={dialogClass}>
        <GrvDialogHeader>
          <div className="grv-dialog-confirm-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className="fa fa-exclamation-triangle fa-2x text-danger" aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">Deleting Server</h3>
                {$headerText}
                <br/>
                <br/>
                <small>This operation cannot be undone. Are you sure?</small>
            </div>
          </div>
        </GrvDialogHeader>
        {isAws  &&
          <GrvDialogContent>
            <Form refCb={e => this.refForm = e}>
              <OverlayHost>
                <div className="row">
                  <InputGroups.AwsAccessKey className="col-sm-12" onChange={this.onAccessKeyChange} />
                  <InputGroups.AwsSecretKey className="col-sm-12" onChange={this.onSecretKeyChange} />
                  <InputGroups.AwsSessionToken className="col-sm-12" onChange={this.onSessionKeyChange}/>
                </div>
              </OverlayHost>
            </Form>
          </GrvDialogContent>
        }
        <GrvDialogFooter>
          <Button className="btn-danger"
            isProcessing={isProcessing}
            onClick={this.onContinue}>
            I understand the consequences, delete this server
          </Button>
          <Button
            onClick={onCancel}
            isPrimary={false}
            isDisabled={isProcessing}
            className="btn btn-white">
            Cancel
          </Button>
        </GrvDialogFooter>
      </GrvDialog>
    );
  }
}

export default RemoveServerDialog;