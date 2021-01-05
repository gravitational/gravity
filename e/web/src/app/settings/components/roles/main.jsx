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
import connect from 'oss-app/lib/connect';
import settingsGetters from 'oss-app/modules/settings/flux/getters';
import userAclGetters from 'oss-app/flux/userAcl/getters';
import {openDeleteDialog} from 'oss-app/modules/settings/flux/actions';
import ConfigItemList from 'oss-app/modules/settings/components/configItemList';
import ConfigDeleteDialog from 'oss-app/modules/settings/components/configDeleteDialog';
import { EmptyList } from 'oss-app/modules/settings/components/items';
import ConfidAddEdit from 'oss-app/modules/settings/components/configAddEdit';
import ChangeTracker from 'oss-app/modules/settings/components/changeTracker';
import Box from 'oss-app/components/common/boxes/box';

// local
import * as roleFlux from './../../flux/roles';
import * as actions from './../../flux/roles/actions';

//import { roleTemplate } from './examples';

class Roles extends React.Component {

  state = {}

  onNewItem = () => {
    this.refTracker.checkIfUnsafedData(()=> {
      let newItem = this.props.store.createItem();
      //newItem = newItem.setContent(roleTemplate);
      actions.setCurRole(newItem);
    });
  }

  onCancelNewItem = () => {
    actions.setCurRole();
  }

  onItemSave = item => {
    actions.saveRole(item);
  }

  onItemClick = item => {
    this.refTracker.checkIfUnsafedData(()=> {
      actions.setCurRole(item);
    })
  }

  onItemDelete = () => {
    openDeleteDialog(this.props.store.curItem);
  }

  componentDidMount(){
    actions.setCurRole();
  }

  render() {
    const { store, saveAttempt, userAclStore } = this.props;
    const items = store.getItems();
    const access = userAclStore.getRoleAccess();
    const canCreate = access.create;
    let curItem = store.getCurItem();

    const props = {
      ref: e => this.refTracker = e,
      className: "grv-settings-tab",
      route: this.props.route
    }

    if(!curItem){
      return box((
        <ChangeTracker {...props}>
          <EmptyList canCreate={canCreate} onClick={this.onNewItem}/>
        </ChangeTracker>
      ), '--empty')
    }

    return box(
      <ChangeTracker {...props}>
        { !curItem.isNew &&
        <ConfigItemList
          canCreate={canCreate}
          btnText="New Role"
          curItem={curItem}
          items={items}
          onNew={this.onNewItem}
          onItemClick={this.onItemClick}
        />
        }
        <ConfidAddEdit
          access={access}
          onCancel={this.onCancelNewItem}
          onDelete={this.onItemDelete}
          onSave={this.onItemSave}
          item={curItem}
          saveAttempt={saveAttempt}/>
        <ConfigDeleteDialog onContinue={actions.deleteRole} />
      </ChangeTracker>
    );
  }
}

const box = (comp, className='') => (
  <Box title="Roles" className={`grv-settings-with-yaml ${className}`}>
    {comp}
  </Box>
)

function mapStateToProps() {
  return {
    userAclStore: userAclGetters.userAcl,
    saveAttempt: settingsGetters.saveConfigAttempt,
    store: roleFlux.getters.store
  }
}

export default connect(mapStateToProps)(Roles)
