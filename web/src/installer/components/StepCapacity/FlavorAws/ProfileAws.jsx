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
import styled from 'styled-components';
import Profile from './../Elements/Profile';
import { FieldSelect } from 'app/installer/components/Fields';
import { Text, Box, LabelInput } from 'shared/components';

export default function AwsItem(props){
  const {
    name,
    instanceTypes,
    instanceTypeFixed,
    count,
    description,
    requirementsText,
    onSetValue,
    ...styles
  } = props;

  const [ selected, setSelected ] = React.useState(() => {
    if(instanceTypeFixed){
      return instanceTypeFixed;
    }

    if(instanceTypes.length === 1){
      return instanceTypes[0];
    }

    return null;
  })

  const options = React.useMemo(() => {
    return instanceTypes.map(i => ({ value: i, label: i }))
  }, [instanceTypes]);

  React.useEffect(() => {
    onSetValue({
      name,
      count,
      instanceType: selected
    })
  }, [selected]);

  function onChange(option){
    setSelected(option.value)
  }

  const showOptions = !(instanceTypes.length === 1 || instanceTypeFixed);

  return (
    <Profile
      count={count}
      requirementsText={requirementsText}
      description={description}
      {...styles}
      flexDirection="row"
    >
    <StyledInstance my={-3} pl="4" py="3" ml="3" width="220px">
      { showOptions && (
        <FieldSelect
          name={`${name}-selected-instance`}
          label={labelText}
          rule={required("Instance type is required")}
          value={{
            value: selected,
            label: selected,
          }}
          options={options}
          onChange={onChange}
        />
      )}
      { !showOptions && (
        <>
          <LabelInput>
            {labelText}
          </LabelInput>
          <Text typography="h5">
            {selected}
          </Text>
        </>
      )}
      </StyledInstance>
    </Profile>
  )
}

const required = errorText => option => () => {
  if(!option || !option.value) {
    return errorText;
  }
}

const labelText = 'Instance type';


const StyledInstance = styled(Box)`
  border-left: 1px solid ${ ({ theme }) => theme.colors.primary.dark };
  flex-shrink: 0;
`