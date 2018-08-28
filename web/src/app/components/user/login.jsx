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
import cfg from 'app/config';
import connect from 'app/lib/connect';
import getters from 'app/flux/user/getters';
import * as actions from 'app/flux/user/actions';
import { Auth2faTypeEnum } from 'app/services/enums';
import Button from 'app/components/common/button';
import Logo from './logo.jsx';
import GoogleAuthInfo from './googleAuthLogo';
import { SsoBtnList, Form } from './userItems';

export class LoginInputForm extends React.Component {
  static propTypes = {  
    authProviders: React.PropTypes.array,
    auth2faType: React.PropTypes.string,    
    onLoginWithSso: React.PropTypes.func.isRequired,
    onLoginWithU2f: React.PropTypes.func.isRequired,
    onLogin: React.PropTypes.func.isRequired,
    attemp: React.PropTypes.object.isRequired
  }

  constructor(props) {
    super(props)
    this.state = {      
      userId: '',
      password: '',
      token: ''      
    }    
  }
  
  onLogin = () => {    
    if (this.isValid()) {
      let { userId, password, token } = this.state;
      this.props.onLogin(userId, password, token);
    }
  }

  onLoginWithSso = ssoProvider => {        
    this.props.onLoginWithSso(ssoProvider);
  }

  onLoginWithU2f = () => {      
    if (this.isValid()) {
      let { userId, password } = this.state;
      this.props.onLoginWithU2f(userId, password);
    }
  }

  onChangeState = (propName, value) => {
    this.setState({
      [propName]: value
    });
  }

  isValid() {
    var $form = $(this.refForm);
    return $form.length === 0 || $form.valid();
  }
  
  needs2fa() {    
    return !!this.props.auth2faType &&
      this.props.auth2faType !== Auth2faTypeEnum.DISABLED;  
  }  

  needsSso() {
    return this.props.authProviders && this.props.authProviders.length > 0;    
  }

  render2faFields() {
    if (!this.needs2fa() || this.props.auth2faType !== Auth2faTypeEnum.OTP) {
      return null;
    }

    return (
      <div className="form-group">
        <input autoComplete="off"          
          value={this.state.token}
          onChange={e => this.onChangeState('token', e.target.value)}
          className="form-control required" name="token"
          placeholder="Two factor token (Google Authenticator)"/>
      </div>
    )
  }

  renderNameAndPassFields() {    
    return (
      <div>
        <div className="form-group">
          <input
            autoFocus
            onChange={e => this.onChangeState('userId', e.target.value)}
            value={this.state.userId}
            className="form-control required"
            placeholder="User name"            
            name="userId"/>
        </div>
        <div className="form-group">
          <input
            onChange={e => this.onChangeState('password', e.target.value)}  
            value={this.state.password}
            type="password"
            name="password"
            className="form-control required"
            placeholder="Password"/>
        </div>
      </div>
    )                    
  }

  renderLoginBtn() {    
    let { isProcessing } = this.props.attemp;    

    let $helpBlock = isProcessing && this.props.auth2faType === Auth2faTypeEnum.UTF ? (
      <div className="help-block">
        Insert your U2F key and press the button on the key
        </div>
    ) : null;

    let onClick = this.props.auth2faType === Auth2faTypeEnum.UTF ?
      this.onLoginWithU2f : this.onLogin;

    return (
      <div>
        <Button
          onClick={onClick}
          isDisabled={isProcessing}
          className="btn btn-primary block full-width m-b">
          Login
        </Button>
        {$helpBlock}
      </div>
    )    
  }
  
  renderSsoBtns() {
    let { authProviders, attemp } = this.props;
    if (!this.needsSso()) {
      return null;
    }

    return (
      <SsoBtnList
        className="m-b-sm"  
        prefixText="Login with "
        isDisabled={attemp.isProcessing}
        providers={authProviders}
        onClick={this.onLoginWithSso}
      />            
    )
  }
  
  render() {
    let { isFailed, message, } = this.props.attemp;                
    return (
      <div>
        <Form refCb={e => this.refForm = e} className="grv-login-input-form">          
          <div>
            {this.renderNameAndPassFields()}
            {this.render2faFields()}
            {this.renderLoginBtn()}
            {this.renderSsoBtns()}
          </div>                                                            
          { isFailed && (<label className="error">{message}</label>)}                      
        </Form>        
      </div>
    );
  }
}

const LoginFooter = props => {
  let { auth2faType } = props;  
  let showGoogleHint = auth2faType === Auth2faTypeEnum.OTP;
  return (
    <div>
      { showGoogleHint && 
          <div className="m-b m-t-md">
            <GoogleAuthInfo/>
          </div>
      }      
    </div>
  )
}

export class Login extends React.Component {
    
  onLogin = (username, password, token) => {
    actions.login(username, password, token);
  }

  onLoginWithSso = ssoProvider => {            
    actions.loginWithSso(ssoProvider.name, ssoProvider.url);      
  }
  
  onLoginWithU2f = (username, password) => {              
    actions.loginWithU2f(username, password);
  }
  
  render() {                

    let hasAnyAuth = !!cfg.auth;

    let formProps = {            
      onLogin: this.onLogin,      
      onLoginWithSso: this.onLoginWithSso,      
      onLoginWithU2f: this.onLoginWithU2f,
      attemp: this.props.attemp,
      authProviders: cfg.getAuthProviders(),      
      auth2faType: cfg.getAuth2faType()
    }

    let footerProps = {      
      auth2faType: cfg.getAuth2faType()      
    }
        
    return (
      <div className="grv-user-login text-center">
        <Logo/>
        <div className="grv-content">
          <div className="p-w-sm">
            <h2 className="m-b-md m-t-xs">{cfg.user.login.headerText}</h2>
            {
              !hasAnyAuth ? <div> You have no authentication options configured </div> 
              :                             
              <div>
                <LoginInputForm {...formProps} />
                <LoginFooter {...footerProps} />
              </div>              
            }
          </div>
        </div>
      </div>
    );
  }
}

function mapStateToProps() {  
  return {            
    attemp: getters.loginAttemp    
  }
}

export default connect(mapStateToProps)(Login);