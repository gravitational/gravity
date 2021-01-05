import React from 'react';
import connect from 'oss-app/lib/connect';
import userAclGetters from 'oss-app/flux/userAcl/getters';
import settingsGetters from 'oss-app/modules/settings/flux/getters';
import {openDeleteDialog} from 'oss-app/modules/settings/flux/actions';
import ConfigItemList from 'oss-app/modules/settings/components/configItemList';
import ConfigDeleteDialog from 'oss-app/modules/settings/components/configDeleteDialog';
import ConfidAddEdit from 'oss-app/modules/settings/components/configAddEdit';
import { EmptyList } from 'oss-app/modules/settings/components/items';
import ChangeTracker from 'oss-app/modules/settings/components/changeTracker';
import Box from 'oss-app/components/common/boxes/box';

import * as authFlux from '../../flux/auth';
import * as actions from '../../flux/auth/actions';

class AuthConnectors extends React.Component {

  state = {}

  onNewItem = () => {
    this.refTracker.checkIfUnsafedData(()=> {
      let newItem = this.props.store.createItem();
      //newItem = newItem.setContent(authTemplate);
      this.onItemClick(newItem);
    });
  }

  onCancelNewItem = () => {
    actions.setCurProvider();
  }

  onItemSave = item => {
    actions.saveAuthProvider(item);
  }

  onItemClick = item => {
    this.refTracker.checkIfUnsafedData(()=> {
      actions.setCurProvider(item);
    })
  }

  onItemDelete = () => {
    openDeleteDialog(this.props.store.curItem);
  }

  componentDidMount(){
    actions.setCurProvider();
  }

  render() {
    const { store, saveAttempt, userAclStore } = this.props;
    const curItem = store.getCurItem();
    const items = store.getItems();
    const access = userAclStore.getConnectorAccess();
    const canCreate = access.create;

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

    const displayItemList = !!curItem && !curItem.isNew;
    const displayYamlEditor = !!curItem;

    return box(
      <ChangeTracker {...props}>
        { displayItemList &&
        <ConfigItemList
          canCreate={canCreate}
          btnText="New Connector"
          curItem={curItem}
          items={items}
          onNew={this.onNewItem}
          onItemClick={this.onItemClick}
        />
        }
        { displayYamlEditor &&
        <ConfidAddEdit
          access={access}
          onCancel={this.onCancelNewItem}
          onDelete={this.onItemDelete}
          onSave={this.onItemSave}
          item={curItem}
          saveAttempt={saveAttempt}/>
        }
        <ConfigDeleteDialog onContinue={actions.deleteAuthProvider } />
      </ChangeTracker>
    );
  }
}

const box = (comp, className='') => (
  <Box title="Auth. Connectors" className={`grv-settings-with-yaml ${className}`}>
    {comp}
  </Box>
)

function mapStateToProps() {
  return {
    saveAttempt: settingsGetters.saveConfigAttempt,
    store: authFlux.getters.store,
    userAclStore: userAclGetters.userAcl
  }
}

export default connect(mapStateToProps)(AuthConnectors);

