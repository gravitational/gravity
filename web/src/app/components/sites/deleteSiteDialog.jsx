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
import Button from 'app/components/common/button';
import Form from 'app/components/common/form';
import * as siteActions from './../../flux/sites/actions';
import siteGetters from './../../flux/sites/getters';
import { ProviderEnum } from 'app/services/enums';
import OverlayHost from 'app/components/common/overlayHost';
import connect from 'app/lib/connect';
import * as InputGroups from 'app/components/inputGroups';

const VALIDATION_WRONG_DEPLOYMENT_NAME = 'Cluster name does not match';

import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialogContent,
  GrvDialog } from './../dialogs/dialog';

class DeleteSiteDialog extends React.Component {

  constructor(props) {
    super(props)
    this.secretKey = '';
    this.accessKey = '';
    this.sessionToken = '';
    this.confirmedSiteId = '';
  }

  static propTypes = {
    attemp: React.PropTypes.object.isRequired,
    siteId: React.PropTypes.string.isRequired,
    onOk: React.PropTypes.func.isRequired,
    provider: React.PropTypes.string.isRequired
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

  onSiteIdChanged = value => {
    this.confirmedSiteId = value;
  }

  onContinue = () => {
    if(this.isValid() && this.props.onOk ){
      this.props.onOk(
        this.confirmedSiteId,
        this.secretKey,
        this.accessKey,
        this.sessionToken);
    }
  }

  isValid() {
    var $form = $(this.refForm);
    return $form.length === 0 || $form.valid();
  }

  componentDidMount() {
    let { siteId } = this.props;
    $.validator.addMethod('grvDeploymentName', value => {
      return siteId === value;
    }, VALIDATION_WRONG_DEPLOYMENT_NAME);

    $(this.refForm).validate({
      rules: {
        deploymentName:{
          required: true,
          grvDeploymentName: true
        }
      }
    })

    $(this.refForm).find('input:first').focus();
  }

  renderAwsContent(){
    return (
      <OverlayHost>
        <div className="row">
          <InputGroups.AwsAccessKey className="col-sm-12" onChange={this.onAccessKeyChange} />
          <InputGroups.AwsSecretKey className="col-sm-12" onChange={this.onSecretKeyChange} />
          <InputGroups.AwsSessionToken className="col-sm-12" onChange={this.onSessionKeyChange}/>
        </div>
      </OverlayHost>
    )
  }

  render() {
    let { siteId, provider, attemp } = this.props;
    let $providerContent = provider === ProviderEnum.AWS ? this.renderAwsContent() : null;
    let { isProcessing } = attemp;

    return (
      <GrvDialog ref="dialog" className="grv-dialog-confirm">
        <GrvDialogHeader>
          <div className="grv-dialog-confirm-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className="fa fa-exclamation-triangle fa-2x text-danger" aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">Are you sure you want to delete this cluster?</h3>
              <small>Deleting cluster <strong>{siteId}</strong> means uninstalling the software and de-provisioning the infrastructure. This cannot be undone.</small>
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogContent>
          <Form refCb={e => this.refForm = e}>
            {$providerContent}
            <div className="form-group">
              <label>Please type in the name of the cluster to confirm</label>
              <input
                onChange={ e => this.onSiteIdChanged(e.target.value) }
                name="deploymentName"
                className="form-control required"
                placeholder="Name of the cluster"/>
            </div>
          </Form>
        </GrvDialogContent>
        <GrvDialogFooter>
          <Button className="btn-danger"
            isProcessing={isProcessing}
            onClick={this.onContinue}>
            I understand the consequences, delete this cluster
          </Button>
          <Button
            onClick={siteActions.closeSiteConfirmDelete}
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


class DeleteSiteDialogContainer extends React.Component {
  render(){
    let { siteToDelete, deleteSiteAttemp, onOk } = this.props;

    if( !siteToDelete ){
      return null;
    }

    let { siteId, provider } = siteToDelete;

    let props = {
      attemp: deleteSiteAttemp,
      siteId,
      provider,
      onOk
    }

    return <DeleteSiteDialog {...props}/>
  }
}

function mapStateToProps() {
  return {
    siteToDelete: siteGetters.siteToDelete,
    deleteSiteAttemp: siteGetters.deleteSiteAttemp
  }
}

export default connect(mapStateToProps)(DeleteSiteDialogContainer);
