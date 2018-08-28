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
import Layout from 'app/components/common/layout';
import { download } from 'app/services/downloader';
import cfg from 'app/config';
import Button from 'app/components/common/button';
import { VersionLabel, Separator, ToolBar } from './items';
import { openSiteConfirmDelete } from 'app/flux/sites/actions';
import { showInfo } from 'app/flux/notifications/actions';
import { ProviderEnum, LinkEnum } from 'app/services/enums';

const MSG_ENABLE_RA = 'Please enable Remote Assistance in order to be able to uninstall the cluster';
const MSG_MANUAL_UNINSTALL = `This cluster cannot be uninstalled from Ops Center, please <a target="_blank" href="${LinkEnum.DOC_CLUSTER_DELETE}"> refer to documentation </a> and follow the steps to manually uninstall the cluster.`;

const AppEndPoints = React.createClass({

  renderEndpoint(endpoint, index){
    let { urls, name, description } = endpoint;

    let $urls = urls.map((url, key) => (
      <div key={key}>
        <a href={url} target="_blank">{url}</a>
      </div>
    ));

    let $description = !description ? null : (
      <small className="help-block no-margins text-primary">
        {description}
      </small>
    )

    return (
      <div key={index} className="grv-site-app-endpoints-item">
        <div>{name}</div>
        {$description}
        <div><small>{$urls}</small></div>
      </div>
    )
  },

  render() {
    let { endpoints, style } = this.props;

    if(endpoints.length === 0){
      return null;
    }

    let $endpoints = endpoints.map(this.renderEndpoint);

    return (
      <div style={style} className="grv-site-app-endpoints m-r">
        <h4>Application endpoints</h4>
        <div className="grv-site-app-endpoints-items">
          {$endpoints}
        </div>
      </div>
    );
  }
})

const CurrentVersion = React.createClass({

  shouldComponentUpdate(){
    return false;
  },

  onOpenPkgImporter(){
    this.refs.pkgUploader.selectPkgFile();
  },

  onUninstall() {
    let {
      id,
      provider,
      isRemoteAccessEnabled
    } = this.props;

    if (provider === ProviderEnum.ONPREM && cfg.isRemoteAccess(id)) {
      showInfo(MSG_MANUAL_UNINSTALL, '', false);
    } else if (!isRemoteAccessEnabled) {
      showInfo(MSG_ENABLE_RA, '');
    } else {
      openSiteConfirmDelete(id);
    }
  },

  render(){
    let {
      isRemoteAccessVisible,
      displayName,
      siteReportUrl,
      version,
      releaseNotes,
      endpoints
    } = this.props;

    let $releaseNotes = null;
    if (releaseNotes) {
      $releaseNotes = (
        <div style={{ flex: "1", minWidth: "300px" }}>
          <div dangerouslySetInnerHTML={{ __html: releaseNotes }} />
        </div>
      )
    }

    let $uninstallBtn = isRemoteAccessVisible && (
      <Button className="btn-danger btn-sm m-l-sm"
        onClick={this.onUninstall}>
        <i className="fa fa-trash m-r-xs"/>
        <span>Uninstall</span>
      </Button>
    )

    return (
      <div className="grv-site-app-cur-ver">
        <ToolBar>
          <h2 className="no-margins">{displayName} <VersionLabel version={version}/></h2>
          <div>
            <Button className="btn-primary btn-sm"
              onClick={ ()=> download(siteReportUrl) } >
              <i className="fa fa-download m-r-xs"/>
              <span>Download Debug Info</span>
            </Button>
            {$uninstallBtn}
          </div>
        </ToolBar>
        <Separator/>
        <Layout.Flex dir="column" style={{ flexWrap: "wrap"}}>
          <AppEndPoints style={{ flex: "1", minWidth: "450px" }} endpoints={endpoints} />
          {$releaseNotes}
        </Layout.Flex>
      </div>
    )
  }
});

export default CurrentVersion;
