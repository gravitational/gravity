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
import { ButtonPrimary, Box } from 'shared/components';
import AdvancedOptions, { Subnets } from './../AdvancedOptions';
import { useValidationContext } from 'app/components/Validation';
import { Danger } from 'shared/components/Alert';
import { useAttempt } from 'shared/hooks';

export default function ProviderOnprem({store, onStart, ...styles }) {
  const { serviceSubnet, podSubnet } = store.state.onprem;
  const validator = useValidationContext()
  const [ attempt, attemptActions ] = useAttempt();
  const { isFailed, isProcessing, message } = attempt;

  function onChangeSubnets({ podSubnet, serviceSubnet}){
    store.setOnpremSubnets(serviceSubnet, podSubnet)
  }

  function onChangeTags(tags){
    store.setClusterTags(tags)
  }

  function onContinue(){
    if(validator.isValid()){
      attemptActions.start();
      const request = store.makeOnpremRequest();
      onStart(request).fail(err => attemptActions.error(err))
    }
  }

  return (
    <Box {...styles}>
      { isFailed && <Danger mb="4">{message}</Danger>}
      <AdvancedOptions onChangeTags={onChangeTags}>
        <Subnets
          serviceSubnet={serviceSubnet}
          podSubnet={podSubnet}
          onChange={onChangeSubnets}
        />
      </AdvancedOptions>
      <ButtonPrimary disabled={isProcessing} mt="4" width="200px" onClick={onContinue}>
        Continue
      </ButtonPrimary>
    </Box>
  );
}