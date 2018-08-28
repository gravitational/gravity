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
import $ from 'jQuery';
import {Select} from 'app/components/common/dropDown';
import Button from 'app/components/common/button';
import Layout from 'app/components/common/layout';
import { UserStatusEnum } from 'app/services/enums'
import { Form } from './../items';
import { UserTokenLink } from './userTokenDialog';
import * as Alerts from 'app/components/common/alerts';

import {  
  GrvDialogContent,
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from 'app/components/dialogs/dialog';

const Label = ({text}) => (
  <label style={{ width: "90px" }} className="text-bold m-t-xs"> {text} </label>
)

const Description = ({text}) => (
  <div className="help-block m-t-xs" >{text}</div>
)

const inputSmallStyle = {
  maxWidth: "300px"
}

const inputLongStyle = {
  maxWidth: "500px"
}

class AddEditUser extends React.Component {
  
  constructor(props){
    super(props);
    const { user } = props;
    this.snapshot = JSON.stringify(user)
    this.isDirty = false;
    this.state = {
      ...user
    }
  }

  static propTypes = {
   saveAttempt: React.PropTypes.object.isRequired,
   inviteAttempt: React.PropTypes.object.isRequired,
   onSave: React.PropTypes.func.isRequired,
   onInvite: React.PropTypes.func.isRequired,
   onCancel: React.PropTypes.func.isRequired,
   roleLabels: React.PropTypes.array,
   user: React.PropTypes.any
  }

  onChangeRoles = newValue => {
    newValue = newValue || '';
    this.state.roles = newValue.split(',').filter(r => r.length > 0)
    this.ensureDirty();
    this.setState(this.state);
  }

  onSave = () => {
    if (!this.isValid()) {                        
      return
    }

    if(this.state.isNew){
      this.props.onInvite({...this.state});  
    }else{
      this.props.onSave({...this.state});
    }                
  }
    
  componentDidMount(){
    this.userNameRef.focus();
  }

  isValid() {    
    const $form = $(this.refForm);
    $form.validate().settings.ignore = [];
    return $form.length === 0 || $form.valid();
  }

  ensureDirty(){
    this.isDirty = JSON.stringify(this.state) !== this.snapshot;
  }

  onChangeField(propName, value) {
    this.state[propName] = value;
    this.ensureDirty();
    this.setState(this.state)
  }

  render() {
    const { onCancel, roleLabels, saveAttempt, inviteAttempt} = this.props;    
    const { roles, isNew } = this.state;
    const invited = this.state.status === UserStatusEnum.INVITED;
    const headerText = isNew ? 'New User' : 'Edit User';
    const isSaveEnabled = isNew || this.isDirty || invited;
    const isProcessing = inviteAttempt.isProcessing || saveAttempt.isProcessing;
    const isFailed = inviteAttempt.isFailed || saveAttempt.isFailed;
    const errorMsg = inviteAttempt.message || saveAttempt.message;

    let btnText = "Save";
    if (this.state.isNew) {
      btnText = "Create invite link";
    } else if (invited) {
      btnText = "Regenerate invite link";
    }
    
    if ( inviteAttempt.isSuccess ){
      return (        
         <UserTokenLink
         tokenType="invite" 
         userToken={inviteAttempt.message} 
         onClose={onCancel}/> 
      )
    }
                   
    return (
    <GrvDialog title="" className="grv-dialog-no-body grv-dialog-md grv-settings-users-addedit grv-settings-dialog-with-errors">                    
        <GrvDialogHeader>
          <div className="grv-settings-users-addedit-header">
            <div className="m-t-xs m-l-xs m-r">
              <i className="fa fa-user fa-2x text-info" aria-hidden="true"></i>
            </div>
            <div>
              <h3>{headerText}</h3>          
            </div>
          </div>         
          { isFailed && <Alerts.Danger>{errorMsg} </Alerts.Danger> }       
        </GrvDialogHeader>      
      <GrvDialogContent>      
        <Form refCb={e => this.refForm = e}>          
          <Layout.Flex dir="row" className="m-t-xs">
            <Label text="Name:" />
            <div className="full-width" style={inputLongStyle}>
              <div>
              <input
                autoFocus
                ref={ e => this.userNameRef = e}
                disabled={!isNew}
                style={inputSmallStyle}
                defaultValue={this.state.userId}
                onChange={e => this.onChangeField('userId', e.target.value)}                
                className="form-control required"
                placeholder="Enter user name" />
              </div>              
            </div>
          </Layout.Flex>
          <Layout.Flex dir="row" className="m-t">
            <Label text="Roles:" />
            <div>
              <div style={inputLongStyle}>
                <Select
                  disabled={invited}
                  className="grv-settings-label-selector2"
                  classRules="required"
                  searchable multi simpleValue clearable={false}
                  value={roles}
                  name="roleSelector"
                  options={roleLabels}
                  placeholder="Click to select a role"
                  onChange={this.onChangeRoles} />
              </div>
              <Description text="Every user must be given at least one role, otherwise he will not be able to login" />
            </div>
          </Layout.Flex>            
        </Form>
        </GrvDialogContent>
        <GrvDialogFooter>           
          <Button className="btn-primary m-r-sm"
            isDisabled={!isSaveEnabled}
            isProcessing={isProcessing}
            onClick={this.onSave}>
            {btnText}
          </Button>
          <Button className="btn-default"
            isDisabled={isProcessing}
            onClick={onCancel}>
            Cancel
          </Button>          
        </GrvDialogFooter>         
      </GrvDialog>
    )
  }
}

export default AddEditUser;
