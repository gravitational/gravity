/*
Copyright 2019 Gravitational, Inc.

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
import { Formik } from 'formik';
import PropTypes from 'prop-types';
import { Auth2faTypeEnum } from './../../services/enums';
import { Card, Input, LabelInput, ButtonPrimary } from 'shared/components';
import * as Alerts from 'shared/components/Alert';

const StatusEnum = {
  PROCESSING: 'processing',
  SUCCESS: 'success',
  FAILED: 'failed'
}

class FormPassword extends React.Component {

  static propTypes = {
    onChangePass: PropTypes.func.isRequired,
    onChangePassWithU2f: PropTypes.func.isRequired
  }

  initialValues = {
    oldPass: '',
    newPass: '',
    newPassConfirmed: '',
    token: '',
  }

  state = {
    status: '',
    error: '',
  };

  submit(values) {
    const { oldPass, newPass, token } = values;
    if (this.props.auth2faType === Auth2faTypeEnum.UTF) {
      return this.props.onChangePassWithU2f(oldPass, newPass);
    }

    return this.props.onChangePass(oldPass, newPass, token);
  }

  onSubmit = (values, { resetForm }) => {
    this.setState({ status: StatusEnum.PROCESSING });
    this.submit(values)
      .then(() => {
        resetForm();
        this.setState({ status: StatusEnum.SUCCESS });
      })
      .fail(err => {
        this.setState({ status: StatusEnum.FAILED, error: err.message })
      })
  }

  onValidate = values => {
    const { oldPass, newPass, token, newPassConfirmed } = values;
    const errors = {};

    if (!oldPass) {
      errors.oldPass = 'Current Password is required';
    }

    if (!newPass) {
      errors.newPass = 'Password cannot be empty';
    } else if (newPass.length < 6) {
      errors.newPass = 'Enter at least 6 characters';
    }

    if (!newPassConfirmed) {
      errors.newPassConfirmed = 'Please confirm your new password'
    }else if (newPassConfirmed !== newPass) {
      errors.newPassConfirmed = 'Password does not match'
    }

    if (this.isOtp() && !token) {
      errors.token = 'Token is required';
    }

    return errors;
  }

  isU2f() {
    return this.props.auth2faType === Auth2faTypeEnum.UTF;
  }

  isOtp() {
    return this.props.auth2faType === Auth2faTypeEnum.OTP;
  }

  renderFields({ values, errors, touched, handleChange }) {
    const isOtpEnabled = this.isOtp();
    const oldPassError = touched.oldPass && errors.oldPass;
    const newPassError = touched.newPass && errors.newPass;
    const newPassConfirmedError = touched.newPassConfirmed && errors.newPassConfirmed;
    const tokenError = touched.token && errors.token;

    return (
      <React.Fragment>
        <LabelInput hasError={Boolean(oldPassError)}>
          {oldPassError || "Current Password"}
        </LabelInput>
        <Input
          hasError={Boolean(oldPassError)}
          value={values.oldPass}
          onChange={handleChange}
          type="password"
          name="oldPass"
          placeholder="Password"
        />
        {isOtpEnabled &&
          <React.Fragment>
            <LabelInput hasError={Boolean(tokenError)}>
              {tokenError || "2nd factor token"}
            </LabelInput>
            <Input
              width="50%"
              hasError={Boolean(tokenError)}
              value={values.token}
              onChange={handleChange}
              type="text"
              name="token"
              placeholder="OTP Token"
            />
          </React.Fragment>
        }
        <LabelInput hasError={Boolean(newPassError)}>
          {newPassError || "New Password"}
        </LabelInput>
        <Input
          hasError={Boolean(newPassError)}
          value={values.newPass}
          onChange={handleChange}
          type="password"
          name="newPass"
          placeholder="New Password"
        />
        <LabelInput hasError={Boolean(newPassConfirmedError)}>
          {newPassConfirmedError || "Confirm Password"}
        </LabelInput>
        <Input
          hasError={Boolean(newPassConfirmedError)}
          value={values.newPassConfirmed}
          onChange={handleChange}
          type="password"
          name="newPassConfirmed"
          placeholder="Confirm Password"
        />
      </React.Fragment>
    )
  }

  renderStatus(status, error) {
    const waitForU2fKeyResponse = status === StatusEnum.PROCESSING && this.isU2f();

    if (status === StatusEnum.FAILED) {
      return (
        <Alerts.Danger>
          {error}
        </Alerts.Danger>
      )
    }

    if (status === StatusEnum.SUCCESS) {
      return (
        <Alerts.Success>
          Your password has been changed
        </Alerts.Success>
      )
    }

    if (waitForU2fKeyResponse) {
      return (
        <Alerts.Info>
          Insert your U2F key and press the button on the key
        </Alerts.Info>
      )
    }

    return null;
  }

  render() {
    const { status, error } = this.state;
    const isProcessing = status === StatusEnum.PROCESSING;
    return (
      <Formik
        validate={this.onValidate}
        onSubmit={this.onSubmit}
        initialValues={this.initialValues}
        >
        {props => (
          <Card as="form" bg="primary.light" width="456px" p="6">
            {this.renderStatus(status, error)}
            {this.renderFields(props)}
            <ButtonPrimary
              block
              disabled={isProcessing}
              size="large"
              type="submit"
              onClick={props.handleSubmit}
              mt={5}>
              Update Password
            </ButtonPrimary>
          </Card>
        )}
      </Formik>
    )
  }
}

export default FormPassword;