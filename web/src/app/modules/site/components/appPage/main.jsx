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
import reactor from 'app/reactor';
import currentSiteGetters from './../../flux/currentSite/getters';
import NewVersion from './newVersion';
import RemoteAssistant from './remoteAssistant';
import CurrentVersion from './currentVersion';
import AppLicense from './appLicense';
import { SiteOpProvider } from './../dataProviders';
import UpdateAppConfirmDialog from './../dialogs/updateAppConfirmDialog';
import DeleteSiteDialog from 'app/components/sites/deleteSiteDialog';
import ServerVersion from 'app/components/common/serverVersion';

import {
  uninstallSite,
  openUpdateAppDialog,
  closeUpdateAppDialog,
  updateSiteApp } from './../../flux/currentSite/actions';

var AppPage = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      appToUpdateTo: currentSiteGetters.appToUpdateTo,
      siteModel: currentSiteGetters.currentSite(),
      endpoints: currentSiteGetters.currentSiteEndpoints,
      newVersions: currentSiteGetters.newVersions,
      updateAttemp: currentSiteGetters.updateAppAttemp,
      updateLicenseAttemp: currentSiteGetters.updateLicenseAttemp
    }
  },

  renderNewVersion(newVersion, index) {
    let { siteModel } = this.state;
    let canUpdate = !siteModel.status2.isProcessing;    
    let siteId = siteModel.id;
    let newVerProps = {
      ...newVersion,
      siteId,
      canUpdate,
      onClick: openUpdateAppDialog      
    }
    return (
      <div key={index} className="row">
        <div className="col-sm-12">
          <div className="ibox">
            <div className="ibox-content">
              <NewVersion {...newVerProps}/>
            </div>
          </div>
        </div>
      </div>
    );
  },

  canUpdateToNewVersion() {
    let { siteModel } = this.state;
    return !siteModel.status2.isProcessing;
  },
       
  render() {
    let {
      updateAttemp,
      appToUpdateTo,
      siteModel,
      endpoints,
      newVersions,
      updateLicenseAttemp} = this.state;

    let $newVersions = newVersions.map(this.renderNewVersion);
    let { license, siteReportUrl, appInfo, id, isRemoteAccessEnabled, provider, isRemoteAccessVisible } = siteModel;
    let currentVersionProps = {      
      ...appInfo,
      id,
      endpoints,
      siteReportUrl,
      isRemoteAccessVisible,
      isRemoteAccessEnabled,
      provider
    }

    return (
      <div className="grv-site-app">
        <div className="">
          <div className="row">
            <div className="col-sm-12">
              <div className="ibox">
                <div className="ibox-content">
                  <CurrentVersion {...currentVersionProps}/>
                </div>
              </div>
            </div>
          </div>
          {
            license &&
            <div className="row">
              <div className="col-sm-12">
                <div className="ibox">
                  <div className="ibox-content">
                    <AppLicense license={license} updateAttemp={updateLicenseAttemp}/>
                  </div>
                </div>
              </div>
            </div>
          }
          { isRemoteAccessVisible &&
            <div className="row">
              <div className="col-sm-12">
                <div className="ibox">
                  <div className="ibox-content">
                    <RemoteAssistant enabled={isRemoteAccessEnabled} />
                  </div>
                </div>
              </div>
            </div>
          }
          {$newVersions}
        </div>
        <div className="grv-footer-server-ver">
          <ServerVersion/>
        </div>
        <SiteOpProvider />
        <DeleteSiteDialog onOk={uninstallSite} />
        <UpdateAppConfirmDialog
          appId={appToUpdateTo}
          onContinue={updateSiteApp}
          onCancel={closeUpdateAppDialog}
          attemp={updateAttemp}
        />
      </div>
    )
  }
});

export default AppPage;
