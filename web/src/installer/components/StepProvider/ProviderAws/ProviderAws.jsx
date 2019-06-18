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
import { ProviderEnum } from 'app/services/enums';
import { ButtonPrimary, Box } from 'shared/components';
import { useAttempt } from 'shared/hooks';
import { Danger } from 'shared/components/Alert'
import service from 'app/installer/services/installer';
import AccessKeys from './AccessKeys';
import ServerSettings from './ServerSettings';
import Validation, { useValidation } from 'app/components/Validation';
import AdvancedOptions from '../AdvancedOptions';


export default function ProviderAws({store, onStart}) {
  const [ attempt, attemptActions ] = useAttempt();
  const { isFailed, message } = attempt;
  const validator = useValidation()

  function onAuthorize({ accessKey, secretKey, sessionToken }){
    return service.verifyAwsKeys({
      packageId: store.state.app.packageId,
      provider: ProviderEnum.AWS,
      accessKey,
      secretKey,
      sessionToken}
    ).then(regions => {
      store.setAwsAccountInfo({
        accessKey,
        secretKey,
        sessionToken,
        regions
      })
    })
  }

  function onChangeServerSettings({selectedRegion, useExisting, selectedKeyPair, selectedVpc}){
    store.setAwsServerSettings({
      useExisting,
      selectedRegion,
      selectedKeyPair,
      selectedVpc
    })
  }

  function onChangeTags(tags){
    store.setClusterTags(tags)
  }

  function onContinue(){
    if (!validator.validate()){
      return;
    }

    const request = store.makeAwsRequest();
    attemptActions.start();
    onStart(request).fail(err => attemptActions.error(err))
  }

  const { authorized, regions } = store.state.aws;
  const continueDisabled = !authorized || attempt.isProcessing;

  return (
    <Validation>
      <Box>
        { isFailed && <Danger children={message}/> }
        { !authorized && <AccessKeys onAuthorize={onAuthorize}/> }
        { authorized  && (<ServerSettings regions={regions} onChange={onChangeServerSettings}/> )}
        <AdvancedOptions mt="4" onChangeTags={onChangeTags}/>
        <ButtonNext mt="4" width="200px" onClick={onContinue} disabled={continueDisabled}>
          Continue
        </ButtonNext>
      </Box>
    </Validation>
  );
}

const ButtonNext = ({ onClick, ...rest }) => {
  const validator = useValidation()
  function onNext(){
    validator.validate() && onClick();
  }

  return <ButtonPrimary onClick={onNext} {...rest} />
}