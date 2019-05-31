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
import { parseCidr } from 'app/lib/paramUtils';
import { Flex, Box, Input, LabelInput } from 'shared/components';
import { useError } from 'app/components/Validation';

const POD_HOST_NUM = 65534;
const INVALID_SUBNET = 'Invalid CIDR format';
const VALIDATION_POD_SUBNET_MIN = `Range cannot be less than ${POD_HOST_NUM}`;

export default function Subnets({ onChange, podSubnet, serviceSubnet, ...styles}){

  function onChangePodnet(e){
    onChange({ podSubnet: e.target.value, serviceSubnet })
  }

  function onChangeServiceSubnet(e){
    onChange({ podSubnet, serviceSubnet: e.target.value })
  }

  const serverError = useError('serviceSubnet', checkCidr(serviceSubnet));
  const podError = useError('podSubnet', checkPod(podSubnet));
  const serviceLabel = serverError ?  serverError : 'Service Subnet';
  const podLabel = podError ?  podError : 'Pod Subnet';

  return (
    <Flex {...styles}>
      <Box flex="1" mr="3">
        <LabelInput hasError={Boolean(serverError)}>
          {serviceLabel}
        </LabelInput>
        <Input
          hasError={Boolean(serverError)}
          mb="3"
          value={serviceSubnet}
          onChange={onChangeServiceSubnet}
          autoComplete="off"
          placeholder="10.0.0.0/16"
        />
      </Box>
      <Box flex="1">
      <LabelInput hasError={Boolean(podError)}>
        {podLabel}
      </LabelInput>
      <Input
        mb="3"
        hasError={Boolean(podError)}
        value={podSubnet}
        onChange={onChangePodnet}
        autoComplete="off"
        placeholder="10.0.0.0/16"
      />
      </Box>
    </Flex>
  )
}

const checkCidr = value => () => {
  if(!parseCidr(value)){
    return INVALID_SUBNET;
  }
}

const checkPod = value => () => {
  const result = parseCidr(value);
  if(result && result.totalHost <= POD_HOST_NUM){
    return VALIDATION_POD_SUBNET_MIN;
  }

  if(!result){
    return INVALID_SUBNET;
  }
}
