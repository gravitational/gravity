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
import { Flex, Box, Text, ButtonPrimary } from 'shared/components';
import { useAttempt } from 'shared/hooks';
import { RadioGroup } from './../../../Radio';
import { useValidationContext } from 'app/components/Validation';
import { Danger } from 'shared/components/Alert';
import { FieldInput } from '../Fields';

const authTypes = [
  {
    value: false,
    label: 'ACCESS/SECRET KEYS'
  },
  {
    value: true,
    label: 'SESSION TOKEN'
  }
]

export default function AccessKeys(props) {
  const [ useSessionToken, setUseSessionToken ] = React.useState(false);
  const validator = useValidationContext()
  const [ accessKey, setAccessKey ] = React.useState();
  const [ secretKey, setSecretKey ] = React.useState();
  const [ sessionToken, setSessionToken ] = React.useState();
  const [ attempt, attemptActions ] = useAttempt();

  const { isFailed, isProcessing, message } = attempt;

  function onChangeSessionToken(e){
    setSessionToken(e.target.value);
  }

  function onChangeAccessKey(e){
    setAccessKey(e.target.value);
  }

  function onChangeSecretKey(e){
    setSecretKey(e.target.value);
  }

  function onAuthorize(){
    if(!validator.isValid()){
      return;
    }

    validator.reset();
    attemptActions.start();
    props.onAuthorize({
      accessKey,
      secretKey,
      sessionToken,
    }).fail(err => attemptActions.error(err))
  }

  function onChangeAuthType(value){
    validator.reset();
    setUseSessionToken(value.value)
  }

  return (
    <Flex width="100%" flexDirection="column">
      <Flex px="3" py="2" height="50px" flex="1" bg="primary.main" alignItems="center" justifyContent="space-between">
        <Text typography="subtitle1" caps>
          Authorize Account
        </Text>
      </Flex>
      <Box p="4" bg="primary.light">
        <RadioGroup
          mb="4"
          name="useSessionToken"
          radioProps={{
            mr: 7,
          }}
          options={authTypes}
          selected={{ value: useSessionToken }}
          onChange={onChangeAuthType}
        />
        { isFailed && <Danger mb="4">{message}</Danger>}
        { useSessionToken && (
          <FieldInput
            name="sessionToken"
            label="Session Token"
            rule={required("Session Token is required")}
            value={sessionToken}
            onChange={onChangeSessionToken}
            name="sessionToken"
            placeholder="FQoDYXdzEHsaDGV2WyeFJbWM6vfdxpngd3VVIIyj0tj7qc9V/qRUVrc8QUdcoOKgkt649VrXP0dK/0X..."
          />
        )}
        { !useSessionToken && (
          <Flex>
            <Box flex="1" mr="3">
              <FieldInput
                name="accessKey"
                rule={required("Access Key is required")}
                value={accessKey}
                onChange={onChangeAccessKey}
                placeholder="AKIAIOSFODNN7EXAMPLE"
                label="Access Key"
              />
            </Box>
            <Box flex="1">
              <FieldInput
                name="secretKey"
                rule={required("Secret Key is required")}
                value={secretKey}
                onChange={onChangeSecretKey}
                label="Secret key" placeholder="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" />
            </Box>
        </Flex>
        )}
        <ButtonPrimary mt="4" onClick={onAuthorize} disabled={isProcessing}>
          AUTHORIZE
        </ButtonPrimary>
      </Box>
    </Flex>
  );
}

const required = errorText => value => () => {
  if(!value) {
    return errorText;
  }
}