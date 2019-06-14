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
import AwsProfile from './ProfileAws';
import PropTypes from 'prop-types';
import ButtonValidate from '../Elements/ButtonValidate';
import * as Alerts from 'shared/components/Alert';
import Validation from 'app/components/Validation';
import { useServices } from 'app/installer/services';
import { useAttempt } from 'shared/hooks';

export default function FlavorAws(props) {
  const { profiles, store } = props;
  const [ attempt, attemptActions ] = useAttempt();
  const service = useServices();

  function onSetProfileValue(profileValues){
    store.setProfileValue(profileValues);
  }

  const $reqItems = profiles.map(profile => {
    const {
      instanceTypes,
      instanceTypeFixed,
      requirementsText,
      count,
      name,
      description
    } = profile;

    return (
      <AwsProfile
        mb="3"
        key={name}
        requirementsText={requirementsText}
        instanceTypes={instanceTypes}
        instanceTypeFixed={instanceTypeFixed}
        count={count}
        name={name}
        description={description}
        onSetValue={onSetProfileValue}
      />
    )
  })

  function onContinue(){
    const request = store.makeStartInstallRequest();
    attemptActions.start();
    service.startInstall(request)
      .done (() => {
        store.setStepProgress();
      })
      .fail(err => {
        attemptActions.error(err)
      })
  }

  const btnDisabled = attempt.isProcessing;
  return (
    <Validation>
      <>
        { attempt.isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
        { attempt.isSuccess && <Alerts.Success>{attempt.message}</Alerts.Success>}
        {$reqItems}
        <ButtonValidate mt="60px" width="200px" disabled={btnDisabled} onClick={onContinue}>
          Continue
        </ButtonValidate>
      </>
    </Validation>
  );
}

FlavorAws.propTypes = {
  profiles: PropTypes.array.isRequired,
  store: PropTypes.object.isRequired,
}