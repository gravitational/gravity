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
import getters from 'app/flux/user/getters';
import * as actions from 'app/flux/user/actions';
import reactor from 'app/reactor';
import Box from 'app/components/common/boxes/box';
import Layout from 'app/components/common/layout';

const Label = ({text}) => ( 
  <label style={{ width: "150px" }} className="text-bold m-t-xs"> {text} </label>
)

export default React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      pswChangeAttemp: getters.pswChangeAttemp
    }
  },

  render() {
    return (                           
      <ChangePassword attemp={this.state.pswChangeAttemp} onClick={actions.changePassword}/>                              
    )
  }
});

const ChangePassword = React.createClass({

  propTypes: {
   attemp: React.PropTypes.object.isRequired,
   onClick: React.PropTypes.func.isRequired
  },
  
  getInitialState(){
    this.hasBeenClicked = false;
    return {
      oldPassword: '',
      newPassword: '',
      newPasswordConfirmed: ''
    }
  },

  componentDidMount(){
    $(this.refs.form).validate({
      rules: {        
        newPassword: {
          minlength: 6,
          required: true
        },
        newPasswordConfirmed: {
          required: true,
          equalTo: this.refs.newPassword
        }
      },
      messages: {
        passwordConfirmed: {
          minlength: $.validator.format('Enter at least {0} characters'), 
          equalTo: 'Enter the same password as above'   
        }
      }
    })
  },

  onClick(e){
    e.preventDefault();
    if(this.isValid()){
      let {oldPassword, newPassword} = this.state;
      this.hasBeenClicked = true;
      this.props.onClick(oldPassword, newPassword);
    }
  },

  isValid(){
    var $form = $(this.refs.form);
    return $form.length === 0 || $form.valid();
  },

  componentWillReceiveProps(nextProps){
    let {isSuccess} = nextProps.attemp;
    if(isSuccess && this.hasBeenClicked){
      // reset all input fields on success
      this.hasBeenClicked = false;
      this.setState(this.getInitialState());
    }
  },

  render() {
    let { isFailed, isProcessing, message } = this.props.attemp;
    let { oldPassword, newPassword, newPasswordConfirmed } = this.state;
    return (
      <Box title="Change Password" className="--no-stretch">
        <div className="m-b" style={{ maxWidth: "500px" }}>
          <form ref="form">
            <Layout.Flex dir="row" className="m-t">
              <Label text="Current Password:" />                                
              <div style={{ flex: "1" }}>
                <input
                  autoFocus
                  type="password"     
                  defaultValue={oldPassword}
                  onChange={e => this.setState({
                    oldPassword: e.target.value
                  })}
                  className="form-control required"
                  placeholder="Current" />                              
              </div>                                                                    
            </Layout.Flex>
            <Layout.Flex dir="row" className="m-t">
              <Label text="New Password:" />                                
              <div style={{ flex: "1" }}>
                <input                  
                  defaultValue={newPassword}
                  onChange={e => this.setState({
                    newPassword: e.target.value
                  })}
                  ref="newPassword"
                  type="password"
                  name="newPassword"
                  className="form-control"
                  placeholder="New" />
              </div>                                                                    
            </Layout.Flex>
            <Layout.Flex dir="row" className="m-t">
              <Label text="Confirm Password:" />                                
              <div style={{ flex: "1" }}>
                <input
                  type="password"
                  defaultValue={newPasswordConfirmed}                  
                  onChange={e => this.setState({
                    newPasswordConfirmed: e.target.value
                  })}
                  name="newPasswordConfirmed"
                  className="form-control"
                  placeholder="Confirm New" />
                { isFailed ? (<label className="error m-t-xs">{message}</label>) : null }
              </div>                                                                    
            </Layout.Flex>                                    
          </form>          
        </div>
        <button disabled={isProcessing} onClick={this.onClick} type="submit" className="btn btn-sm btn-primary block" >Update</button>
      </Box>  
    )
  }
});
