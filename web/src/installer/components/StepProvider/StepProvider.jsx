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
import { FieldInput } from 'app/installer/components/Fields';
import { StepLayout } from '../Layout';
import ProviderOnprem from './ProviderOnprem';
import { useInstallerContext } from './../store';

export default function StepProvider() {
  const store = useInstallerContext();
  const { clusterName } = store.state;

  function onChangeName(name){
    store.setClusterName(name);
  }

  function onStart(request){
    return service.createCluster(request).then(clusterName => {
      history.push(cfg.getInstallerProvisionUrl(clusterName), true);
    })
  }

  return (
    <StepLayout title="Name your cluster">
      <FieldInput
        mb="2"
        placeholder="prod.example.com"
        autoFocus
        rule={required}
        value={clusterName}
        onChange={e => onChangeName(e.target.value )}
        label="Cluster Name"
      />
      <ProviderOnprem store={store} onStart={onStart} />
    </StepLayout>
  );
}

const required = value => () => (
  {
    valid: !!value,
    message: 'Cluster name is required'
  }
);