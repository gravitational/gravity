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
import PropTypes from 'prop-types';
import { Formik } from 'formik';
import { Auth2faTypeEnum } from 'app/services/enums';
import { Text, Card, Input, LabelInput, ButtonLink, ButtonPrimary, Flex, Box } from '../';
import * as Alerts from './../Alert';
import TwoFAData from './TwoFaInfo';

const U2F_ERROR_CODES_URL = 'https://developers.yubico.com/U2F/Libraries/Client_error_codes.html';

const needs2fa = auth2faType => !!auth2faType && auth2faType !== Auth2faTypeEnum.DISABLED;

export class FormInvite extends React.Component {

  static propTypes = {
    submitBtnText: PropTypes.string,
    auth2faType: PropTypes.string,
    authType: PropTypes.string,
    onSubmitWithU2f: PropTypes.func.isRequired,
    onSubmit: PropTypes.func.isRequired,
    attempt: PropTypes.object.isRequired,
    user:  PropTypes.string.isRequired,
    qr:  PropTypes.string
  }

  static defaultProps = {
    submitBtnText: 'Submit'
  }

  initialValues = {
    password: '',
    passwordConfirmed: '',
    token: ''
  }

  onValidate = values => {
    const { password, passwordConfirmed } = values;
    const errors = {};

    if (!password) {
      errors.password = 'Password is required';
    } else if (password.length < 6) {
      errors.password = 'Enter at least 6 characters';
    }

    if (!passwordConfirmed) {
      errors.passwordConfirmed = 'Please confirm your password'
    }else if (passwordConfirmed !== password) {
      errors.passwordConfirmed = 'Password does not match'
    }

    if (this.isOTP() && !values.token) {
      errors.token = 'Token is required';
    }

    return errors;
  }

  onSubmit = values => {
    const { password, token } = values;
    if (this.props.auth2faType === Auth2faTypeEnum.UTF) {
      this.props.onSubmitWithU2f(password);
    } else {
      this.props.onSubmit(password, token);
    }
  }

  renderNameAndPassFields({ values, errors, touched, handleChange }) {
    const passError = touched.password && errors.password;
    const passConfirmedError = touched.passwordConfirmed && errors.passwordConfirmed;
    const tokenError = errors.token && touched.token;
    const user = this.props.user;

    return (
      <React.Fragment>
        <Text typography="h4" breakAll mb={3}>
          {user}
        </Text>
        <LabelInput hasError={passError}>
          {passError || "Password"}
        </LabelInput>
        <Input
          hasError={passError}
          value={values.password}
          onChange={handleChange}
          type="password"
          name="password"
          placeholder="Password"
        />
        <LabelInput hasError={passConfirmedError}>
          {passConfirmedError || "Confirm Password"}
        </LabelInput>
        <Input
          hasError={passConfirmedError}
          value={values.passwordConfirmed}
          onChange={handleChange}
          type="password"
          name="passwordConfirmed"
          placeholder="Password"
        />
        {this.isOTP() && (
       <Flex flexDirection="column">
        <LabelInput mb={2} hasError={tokenError}>
          {(tokenError && errors.token) || "Two factor token"}
        </LabelInput>
        <Flex mb="4">
          <Input width="160px" id="token" fontSize={0}
            hasError={tokenError}
            autoComplete="off"
            value={values.token}
            mb="0"
            onChange={handleChange}
            placeholder="123 456"
          />
          <ButtonLink width="100%" kind="secondary" target="_blank" size="small" href="https://support.google.com/accounts/answer/1066447?co=GENIE.Platform%3DiOS&hl=en&oco=0">
            Download Google Authenticator
          </ButtonLink>
        </Flex>
      </Flex>

        ) }
      </React.Fragment>
    )
  }

  isOTP() {
    let { auth2faType } = this.props;
    return needs2fa(auth2faType) && auth2faType === Auth2faTypeEnum.OTP;
  }

  renderSubmitBtn(onClick) {
    const { isProcessing } = this.props.attempt;
    const { submitBtnText } = this.props;
    const $helpBlock = isProcessing && this.props.auth2faType === Auth2faTypeEnum.UTF ? (
        "Insert your U2F key and press the button on the key"
    ) : null;

    const isDisabled = isProcessing;

    return (
      <>
        <ButtonPrimary
          width="100%"
          disabled={isDisabled}
          size="large"
          type="submit"
          onClick={onClick}
          mt={3}>
          {submitBtnText}
      </ButtonPrimary>
      {$helpBlock}
      </>
    )
  }

  render() {
    const { auth2faType, qr, attempt } = this.props;
    const { isFailed, message } = attempt;
    const $error = isFailed ? <ErrorMessage message={message} /> : null;
    const needs2FAuth = needs2fa(auth2faType);
    const boxWidth = (needs2FAuth ? 720 : 464) + 'px';

    let $2FCode = null;
    if(needs2FAuth) {
      $2FCode = (
        <Box flex="1" bg="primary.main" p="6">
          <TwoFAData auth2faType={auth2faType} qr={qr} />
        </Box>
      );
    }

    return (
      <Formik
        validate={this.onValidate}
        onSubmit={this.onSubmit}
        initialValues={this.initialValues}
      >
        {
          props => (
            <Card as="form" bg="primary.light" my={6} mx="auto" width={boxWidth}>
              <Flex>
                <Box flex="3" p="6">
                  {$error}
                  {this.renderNameAndPassFields(props)}
                  {this.renderSubmitBtn(props.handleSubmit)}
                </Box>
                {$2FCode}
              </Flex>
            </Card>
          )
        }
      </Formik>
    )
  }
}

export const ErrorMessage = ({ message = '' }) => {
  // quick fix: check if error text has U2F substring
  const showU2fErrorLink = message.indexOf('U2F') !== -1;
  return (
    <Alerts.Danger>
      {message}
      {showU2fErrorLink && (
        <Text typography="body2">
          click <a target="_blank" href={U2F_ERROR_CODES_URL}>here</a> to learn more about U2F error codes
        </Text>
        )
      }
    </Alerts.Danger>
  )
}

export default FormInvite;