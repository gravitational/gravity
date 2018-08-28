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
import userAclGetters from 'app/flux/userAcl/getters';
import Box from 'app/components/common/boxes/box';

import settingsGetters from './../../flux/getters';
import {openDeleteDialog} from '../../flux/actions';
import ConfigItemList from './../configItemList';
import ConfigDeleteDialog from './../configDeleteDialog';
import ConfidAddEdit from './../configAddEdit';
import { EmptyList } from './../items';
import ChangeTracker from './../changeTracker';
import * as logFwrds from '../../flux/logForwarders';
import * as actions from '../../flux/logForwarders/actions';

class LogForwarders extends React.Component {

  state = {}

  onNewItem = () => {
    this.refTracker.checkIfUnsafedData(()=> {    
      let newItem = this.props.store.createItem();
      //newItem = newItem.setContent(authTemplate);      
      this.onItemClick(newItem);    
    });
  }
  
  onCancelNewItem = () => {    
    actions.setCurFwrd();
  }

  onItemSave = item => {    
    actions.saveForwarder(item);
  }

  onItemClick = item => {    
    this.refTracker.checkIfUnsafedData(()=> {
      actions.setCurFwrd(item);    
    }) 
  }

  onItemDelete = () => {
    openDeleteDialog(this.props.store.curItem);
  }
    
  componentDidMount(){    
    actions.setCurFwrd();
  }
  
  render() {        
    const { store, saveAttempt, userAclStore } = this.props;                
    const curItem = store.getCurItem();
    const items = store.getItems();       
    const access = userAclStore.getLogForwarderAccess();
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
        <ConfigDeleteDialog onContinue={actions.deleteLogForwarder} />                                          
      </ChangeTracker>                    
    );
  }    
}

const box = (comp, className='') => (
  <Box title="Log Forwarders" className={`grv-settings-with-yaml ${className}`}>
    {comp}
  </Box>
)

function mapStateToProps() {
  return {    
    saveAttempt: settingsGetters.saveConfigAttempt,    
    store: logFwrds.getters.store,
    userAclStore: userAclGetters.userAcl
  }  
}

export default connect(mapStateToProps)(LogForwarders);

