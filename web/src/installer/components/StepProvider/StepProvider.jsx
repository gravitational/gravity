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
import cfg from 'app/config';
import history from 'app/services/history';
import service from 'app/installer/services/installer';
import { ProviderEnum } from 'app/services/enums';
import ClusterName from './ClusterName';
import ProviderSelector from './ProviderSelector';
import { StepLayout } from '../Layout';
import ProviderAws from './ProviderAws';
import ProviderOnprem from './ProviderOnprem';
import { useInstallerContext } from './../store';

export default function StepProvider() {
  const store = useInstallerContext();
  const { clusterName, selectedProvider } = store.state;
  const { providers } = store.state.config;

  function onChangeProvider(providerName){
    store.setProvider(providerName)
  }

  function onChangeClusterName(name){
    store.setClusterName(name);
  }

  function onStart(request){
    return service.createCluster(request).then(clusterName => {
      history.push(cfg.getInstallerProvisionUrl(clusterName), true);
    })
  }

  return (
    <StepLayout title="Choose a Provider">
      <ClusterName
        value={clusterName}
        onChange={onChangeClusterName}
      />
      <ProviderSelector mb="5"
        value={selectedProvider}
        options={providers}
        onChange={onChangeProvider}
      />
      { selectedProvider === ProviderEnum.AWS && <ProviderAws store={store} onStart={onStart} mb="3"/>}
      { selectedProvider === ProviderEnum.ONPREM && <ProviderOnprem  mb="4" store={store} onStart={onStart} /> }
    </StepLayout>
  );
}
