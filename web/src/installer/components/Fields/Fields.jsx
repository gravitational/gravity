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
import { Box, Input, LabelInput } from 'shared/components';
import Select from 'app/components/Select';
import { useError } from 'app/components/Validation';

export const FieldInput = ({ rule, name, value, label, onChange, ...styles}) => {
  const error = useError(name, rule(value));
  const hasError = Boolean(error);
  const labelText = hasError ? error : label;
  return (
    <Box {...styles}>
      { label && (
          <LabelInput hasError={hasError}>
            {labelText}
          </LabelInput>
        )}
      <Input
        hasError={hasError}
        mb="3"
        value={value}
        autoComplete="off"
        onChange={onChange}
      />
    </Box>
  )
}

export const FieldSelect = ({ rule, name, label, value, options, onChange, ...styles}) => {
  const error = useError(name, rule(value));
  const hasError = Boolean(error);
  const labelText = hasError ? error : label;
  return (
    <Box {...styles}>
      { label && (
        <LabelInput hasError={hasError}>
          {labelText}
        </LabelInput>
      )}
      <Select
        hasError={hasError}
        isSimpleValue
        isSearchable={false}
        clearable={false}
        value={value}
        onChange={onChange}
        options={options}
      />
    </Box>
  )
}