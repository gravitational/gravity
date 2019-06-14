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
import { Flex, Box, Text, LabelInput } from 'shared/components';
import { FieldSelect } from 'app/installer/components/Fields';
import { RadioGroup } from 'app/installer/components/Radio';

export const  CREATE_NEW_VPC_OPTION_VALUE = '__CREATE__NEW__VPC__';

export default function RegionSettings(props) {
  const [ regionOptions ] = React.useState(() => {
    return makeRegionOptions(props.regions);
  })

  const [ selectedRegion, setRegion ] = React.useState(()=>{
    return regionOptions[0];
  });

  const [ useExisting, setUseExisting ] = React.useState(false);
  const [ selectedKeyPair, setKeyPair ] = React.useState({});
  const [ selectedVpc, setVpc ] = React.useState({});
  const [ vpcOptions, setVpcOptions ] = React.useState([]);
  const [ keyPairOptions, setKeyPairOptions ] = React.useState([]);

  // preselect vpc and keypair when region changes
  React.useEffect(() => {
    if(selectedRegion && selectedRegion.vpcs){
      setVpcOptions(selectedRegion.vpcs);
      setKeyPairOptions(selectedRegion.keyPairs);

      const vpc = selectedRegion.vpcs.find(r => r.isDefault === true) || {};
      setVpc(vpc);
      setKeyPair(selectedRegion.keyPairs[0])
    }
  }, [selectedRegion]);

  // notify parent component about changes
  React.useEffect(() => {
    props.onChange({
      useExisting,
      selectedKeyPair: selectedKeyPair.value,
      selectedRegion: selectedRegion.value,
      selectedVpc: selectedVpc.value === CREATE_NEW_VPC_OPTION_VALUE ? null : selectedVpc.value,
    })
  }, [selectedRegion, selectedVpc, selectedKeyPair, useExisting]);

  function onSelectRegion(option){
    setRegion(option);
  }

  function onSelectKeyPair(option){
    setKeyPair(option)
  }

  function onSelecVpc(option){
    setVpc(option)
  }

  function onChangeInstallType(option){
    setUseExisting(option.value)
  }

  return (
    <Flex width="100%" flexDirection="column">
      <Flex px="3" py="2" flex="1" bg="primary.main" alignItems="center" justifyContent="space-between">
        <Text typography="subtitle1" caps>
          Server Settings
        </Text>
      </Flex>
      <Box p="4" bg="primary.light">
        <FieldSelect label="Select your server region" name="selectedRegion"
          rule={required("Server region is required")}
          value={selectedRegion}
          options={regionOptions}
          onChange={onSelectRegion}
        />
        <Flex mb="2">
          <FieldSelect mr="3" label="Select your key pair"
            flex="1"
            rule={required("Key Pair is required")}
            value={selectedKeyPair}
            onChange={onSelectKeyPair}
            options={keyPairOptions}
          />
          <FieldSelect label="Select your vpc"
            flex="1"
            rule={required("Your vpc is required")}
            options={vpcOptions}
            onChange={onSelecVpc}
            value={selectedVpc}
          />
        </Flex>
        <Box>
          <LabelInput>
              Installation Type
            </LabelInput>
            <RadioGroup
              name="installation_type"
              radioProps={{
                mr: 7,
              }}
              options={installTypes}
              selected={{ value: useExisting }}
              onChange={onChangeInstallType}
            />
        </Box>
      </Box>
    </Flex>
  );
}

const installTypes = [
  {
    value: false,
    label: 'PROVISION NEW SERVERS'
  },
  {
    value: true,
    label: 'USE EXISTING SERVERS'
  }
]

const required = message => option => () => {
  const valid = option && option.value;
  return {
    valid,
    message
  }
}

const newVpcOption = {
  value: CREATE_NEW_VPC_OPTION_VALUE,
  label: 'Create new'
};

function makeRegionOptions(regions){
  return regions.map( r => ({
    value: r.name,
    label: r.name,
    keyPairs: r.keyPairs.map( k => ({
      value: k.name,
      label: k.name
    })),
    vpcs: mapVpcOptions(r.vpcs || [])
  }))
}

function mapVpcOptions(vpcs){
  const vpcOptions = vpcs.map(v => ({
    value: v.id,
    label: v.name,
    isDefault: v.isDefault
  }))

  vpcOptions.unshift(newVpcOption)
  return vpcOptions;
}