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
import PropTypes from 'prop-types';
import { Flex, Box, LabelInput } from 'shared/components';
import { RadioGroup } from './../../Radio';

export default function ProviderSelector(props){
  const { value, options, onChange, ...styles } = props;

  const radioOptions = options.map(o => ({
    value: o,
    label: getProviderDisplayName(o)
  }));

  const selected = {
    value
  }

  function onChangeOption(option){
    onChange(option.value)
  }

  return (
    <Box {...styles}>
      <LabelInput>
        Choose a provider
      </LabelInput>
      <Flex>
        <RadioGroup
          name="providers"
          radioProps={{
            mr: 7,
          }}
          options={radioOptions}
          selected={selected}
          onChange={onChangeOption}
        />
      </Flex>
    </Box>
  )
}

function getProviderDisplayName(name){
  switch(name){
    case ProviderEnum.AWS:
      return 'AMAZON (AWS)';
    case ProviderEnum.ONPREM:
      return 'BARE METAL';
    case ProviderEnum.AZURE:
      return 'MICROSOFT AZURE';
    default:
      return name;
  }
}

ProviderSelector.propTypes = {
  options: PropTypes.array.isRequired,
  value: PropTypes.string
}