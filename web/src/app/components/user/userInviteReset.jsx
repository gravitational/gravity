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
import classnames from 'classnames';
import connect from 'app/lib/connect';
import cfg from 'app/config';
import { Auth2faTypeEnum, LinkEnum } from 'app/services/enums';
import getters from 'app/flux/user/getters';
import * as actions from 'app/flux/user/actions';
import GoogleAuthInfo from './googleAuthLogo';
import {GravitationalLogo} from './../icons.jsx';
import { ExpiredLink } from './../msgPage';
import Button from '../common/button';
import { Form } from './userItems';

class InputForm extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      psw: '',
      pswConfirmed: '',
      token: ''
    }
  }

  static propTypes = {
    auth2faType: React.PropTypes.string.isRequired,
    userName: React.PropTypes.string.isRequired,
    onSubmit: React.PropTypes.func.isRequired,
    onSubmitWithU2f: React.PropTypes.func.isRequired
  }

  onChangeState = (propName, value) => {
    this.setState({
      [propName]: value
    });
  }

  onSubmit = () => {
    if (!this.isValid()) {
      return;
    }

    if (this.props.auth2faType === Auth2faTypeEnum.UTF) {
      this.props.onSubmitWithU2f(this.state.psw);
    } else {
      this.props.onSubmit(this.state.psw, this.state.token);
    }
  }

  componentDidMount() {
    $(this.refForm).validate({
      rules:{
        password:{
          minlength: 6,
          required: true
        },
        passwordConfirmed:{
          required: true,
          equalTo: this.refs.password
        }
      },

      messages: {
  			passwordConfirmed: {
  				minlength: $.validator.format('Enter at least {0} characters'),
  				equalTo: 'Enter the same password as above'
  			}
      }
    })
  }

  isValid() {
    let $form = $(this.refForm);
    return $form.length === 0 || $form.valid();
  }

  render2faFields() {
    if (this.props.auth2faType === Auth2faTypeEnum.OTP) {
      return (
        <div className="form-group">
          <input
            autoComplete="off"
            name="token"
            onChange={e => this.onChangeState('token', e.target.value)}
            value={this.state.token}
            className="form-control required"
            placeholder="Two factor token (Google Authenticator)" />
        </div>
      )
    }

    return null;
  }

  render() {
    const { isFailed, isProcessing, message } = this.props.attemp;
    return (
      <Form refCb={e => this.refForm = e} className="grv-user-invite-input-form">
        <div className="">
          <div className="form-group">
            <input
              disabled
              onChange={e => this.onChangeState('userName', e.target.value)}
              value={this.props.userName}
              name="userName"
              className="form-control required"
              placeholder="User name"/>
          </div>
          <div className="form-group">
            <input
              autoFocus
              value={this.state.psw}
              onChange={e => this.onChangeState('psw', e.target.value)}
              ref="password"
              type="password"
              name="password"
              className="form-control"
              placeholder="Password" />
          </div>
          <div className="form-group">
            <input
              onChange={e => this.onChangeState('pswConfirmed', e.target.value)}
              value={this.state.pswConfirmed}
              type="password"
              name="passwordConfirmed"
              className="form-control"
              placeholder="Password confirm"/>
          </div>
          {this.render2faFields()}
          <Button
            onClick={this.onSubmit}
            className="btn btn-primary block full-width m-b"
            isDisabled={isProcessing}
            onClick={this.onSubmit} > Continue </Button>
          { isFailed ? (<label className="grv-user-invite-form-error">{message}</label>) : null }
        </div>
      </Form>
    );
  }
}

const InputFormFooter = ({ auth2faType }) => {
  if (auth2faType === Auth2faTypeEnum.OTP) {
    return <GoogleAuthInfo />
  }

  if (auth2faType === Auth2faTypeEnum.UTF) {
    return (
      <div className="text-muted m-t-n-sm grv-user-invite-utf-info">
        <small>Click
          <a target="_blank" href={LinkEnum.U2F_HELP_URL}> here </a>
          to learn more about U2F 2-Step Verification.
        </small>
      </div>
    )
  }

 return null;
}

const TwoFAInfo = ({ qrCode, auth2faType }) => {
  if (auth2faType === Auth2faTypeEnum.OTP) {
    return (
      <div className="grv-flex-column grv-user-invite-barcode">
        <h4>Scan bar code for auth token <br /> <small>Scan below to generate your two factor token</small></h4>
        <img className="img-thumbnail" src={`data:image/png;base64,${qrCode}`} />
      </div>
    )
  }

  if (auth2faType === Auth2faTypeEnum.UTF) {
    return (
      <div className="grv-flex-column grv-user-invite-utf-info">
        <h3>Insert your U2F key </h3>
        <div className="m-t-md">Press the button on the U2F key after you press the continue button</div>
    </div>
    )
  }

  return null;
}

class UserInviteReset extends React.Component {

  constructor(props) {
    super(props);
  }

  static propTypes = {
    params: React.PropTypes.shape({
      token: React.PropTypes.string.isRequired
    })
  }

  onSubmit = (psw, hotpToken) => {
    const { isInvite, isReset, token } = this.props.userRequestInfo;
    if (isInvite) {
      actions.completeInviteWith2fa(psw, hotpToken, token);
    } else if (isReset) {
      actions.completeResetWith2fa(psw, hotpToken, token);
    }
  }

  onSubmitWithU2f = psw => {
    const { isInvite, userName, token } = this.props.userRequestInfo;
    if (isInvite) {
      actions.completeInviteWithU2f(userName, psw, token);
    }else{
      actions.completeResetWithU2f(userName, psw, token);
    }
  }

  componentDidMount(){
    actions.fetchUserToken(this.props.params.token);
  }

  getTitle() {
    const { isReset } = this.props.userRequestInfo;
    const { newPasswordHeaderText, inviteHeaderText } = cfg.user.completeRequest;
    if (isReset) {
      return newPasswordHeaderText;
    }

    return inviteHeaderText;
  }

  render() {
    const {
      completeUserTokenAttemp,
      userRequestInfo,
      fetchUserTokenAttemp } = this.props;

    if (fetchUserTokenAttemp.isFailed) {
      return <ExpiredLink />;
    }

    if (!fetchUserTokenAttemp.isSuccess) {
      return null;
    }

    const title = this.getTitle();
    const auth2faType = cfg.getAuth2faType();
    const inputFormProps = {
      onSubmit: this.onSubmit,
      onSubmitWithU2f: this.onSubmitWithU2f,
      attemp: completeUserTokenAttemp,
      userName: userRequestInfo.userName,
      auth2faType
    }

    const secondFactorInfoProps = {
      qrCode: userRequestInfo.qrCode,
      auth2faType
    }

    const containerClass = classnames('grv-user-invite-content',
      needs2fa(auth2faType) && '--with-2fa-data'
    );

    return (
      <div className="grv-user-invite text-center">
        <GravitationalLogo />
        <div className={containerClass}>
          <h3>{title}</h3>
          <div className="grv-flex">
            <div className="grv-flex-column">
              <InputForm {...inputFormProps}/>
              <InputFormFooter auth2faType={auth2faType} />
            </div>
            <TwoFAInfo {...secondFactorInfoProps} />
          </div>
        </div>
      </div>
    );
  }
}

const needs2fa = auth2faType => !!auth2faType && auth2faType !== Auth2faTypeEnum.DISABLED;

function mapStateToProps() {
  return {
    userRequestInfo: getters.userRequestInfo,
    fetchUserTokenAttemp: getters.fetchUserTokenAttemp,
    completeUserTokenAttemp: getters.completeUserTokenAttemp
  }
}

export default connect(mapStateToProps)(UserInviteReset);