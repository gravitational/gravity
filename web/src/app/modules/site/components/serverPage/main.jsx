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
import connect from 'app/lib/connect';
import { Table, Column, Cell, TextCell } from 'app/components/common/tables/table.jsx';
import { ProviderEnum } from 'app/services/enums';
import { SiteOpProvider, SiteServerProvider }  from './../dataProviders.jsx';
import getters from './../../flux/servers/getters';
import * as actions from './../../flux/servers/actions';
import {openServerTerminal } from './../../flux/actions';
import currentSiteGetters from './../../flux/currentSite/getters';
import Provisioner from './provision/main';
import RemoveServerDialog from './../dialogs/removeServerDialog.jsx';
import OverlayHost from 'app/components/common/overlayHost';
import * as Dropdowns from 'app/components/common/dropDownMenu';

const makeLoginInputKeyPressHandler = serverId => e =>  {
  if (e.key === 'Enter' && e.target.value) {
    openServerTerminal({ serverId, login: e.target.value });
  }
}

const makeLoginOnClickHandler = (serverId, login) => () => {
  openServerTerminal({ serverId, login });
}

const PrivateIPCell = ({ rowIndex, data, ...props }) => {
  const { sshLogins, advertiseIp, id, hostname } = data[rowIndex];
  const $lis = [];
  for (var i = 0; i < sshLogins.length; i++){
    $lis.push(
      <Dropdowns.MenuItemLogin key={i}
        onClick={makeLoginOnClickHandler(id, sshLogins[i])}
        text={sshLogins[i]}/>
    );
  }

  return (
    <Cell {...props}>
      <Dropdowns.Menu text={advertiseIp}>
        <Dropdowns.MenuItemTitle text="SSH login as:"/>
        {$lis}
        <Dropdowns.MenuItemLoginInput onKeyPress={makeLoginInputKeyPressHandler(id)} />
        <Dropdowns.MenuItemDivider/>
        <Dropdowns.MenuItemDelete onClick={ () => actions.showRemoveServerConfirmation(hostname) } />
      </Dropdowns.Menu>
    </Cell>
  );
}

class ServerPage  extends React.Component {
  render() {
    const { servers, currentSite, serverToRemove, removeServerAttemp } = this.props;
    const { provider } = currentSite;
    const isAws = provider === ProviderEnum.AWS;
    return (
      <OverlayHost>
        <div className="grv-site-servers grv-page">
          <div className="">
            <div className="row">
              <div className="col-sm-12">
                <Provisioner/>
              </div>
            </div>
            <div className="row">
              <div className="col-sm-12">
                <div className="grv-site-table">
                  <Table data={servers} rowCount={servers.length}>
                    <Column
                      header={<Cell>Private IP</Cell> }
                      cell={<PrivateIPCell /> }
                    />
                    <Column
                      columnKey="publicIp"
                      header={<Cell>Public IP</Cell> }
                      cell={<TextCell/> }
                    />
                    <Column
                      columnKey="hostname"
                      header={<Cell>Hostname</Cell> }
                      cell={<TextCell/> }
                    />
                    <Column
                      columnKey="displayRole"
                      header={<Cell>Profile</Cell> }
                      cell={<TextCell/> }
                    />
                    { isAws ?
                      <Column
                        columnKey="instanceType"
                        header={<Cell>Instance Type</Cell> }
                        cell={<TextCell /> }
                      /> : null }
                  </Table>
                </div>
              </div>
            </div>
          </div>
          {
            serverToRemove  ?
              <RemoveServerDialog
                attemp={removeServerAttemp}
                provider={provider}
                hostname={serverToRemove}
                onContinue={actions.startShrinkOperation}
                onCancel={actions.hideRemoveServerConfirmation}
              />
            : null
          }
          <SiteOpProvider/>
          <SiteServerProvider/>
        </div>
      </OverlayHost>
    )
  }
}

const mapStateToProps = () => ({
  servers: getters.servers,
  serverToRemove: getters.serverToRemove,
  currentSite: currentSiteGetters.currentSite(),
  removeServerAttemp: getters.removeServerAttemp
})

export default connect(mapStateToProps)(ServerPage);
