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
import { Input, LabelInput } from 'shared/components';
import Select from 'react-select';
import { useError } from 'app/components/Validation';

export const FieldInput = ({ rule, name, value, label, ...rest}) => {
  const error = useError(name, rule(value));
  const hasError = Boolean(error);
  const labelText = hasError ? error : label;
  return (
    <>
      { label && (
          <LabelInput hasError={hasError}>
            {labelText}
          </LabelInput>
        )}
      <Input
        hasError={hasError}
        mb="3"
        autoComplete="off"
        {...rest}
      />
    </>
  )
}

export const FieldSelect = ({ rule, name, label, value, ...rest}) => {
  const error = useError(name, rule(value));
  const hasError = Boolean(error);
  const labelText = hasError ? error : label;
  return (
    <>
      { label && (
        <LabelInput hasError={hasError}>
          {labelText}
        </LabelInput>
      )}
      <StyledSelect hasError={hasError}>
        <Select
          className="react-select-container"
          classNamePrefix="react-select"
          isSimpleValue
          isSearchable={false}
          clearable={false}
          value={value}
          {...rest}
        />
      </StyledSelect>
    </>
  )
}

const StyledSelect = styled.div`
  .react-select__control,
  .react-select__control--is-focused {
    ${ ({ hasError, theme}) => {
      if(hasError){
        return {
          borderRadius: 'inherit !important',
          borderWidth: '2px !important',
          backgroundColor: `${theme.colors.error.light} !important`,
          border: `2px solid ${theme.colors.error.main}  !important`,
        }
      }
    }}
  }

  .react-select-container {
    box-shadow: inset 0 2px 4px rgba(0,0,0,.24);
    box-sizing: border-box;
    border: none;
    display: block;
    font-size: 16px;
    outline: none;
    width: 100%;
    color: rgba(0,0,0,0.87);
    background-color: #FFFFFF;
    margin-bottom: 24px;
    border-radius: 4px;
  }

  .react-select__menu{
    margin-top: 0px;
  }

  react-select__menu-list {
  }

  .react-select__indicator-separator{
    display: none;
  }

  .react-select__control {
    &:hover {
      border-color: transparent;
    }
  }

  .react-select__control--is-focused {
    background-color: transparent;
    border-color: transparent;
    border-radius: 4px;
    border-style: solid;
    border-width: 1px;
    box-shadow: none;
  }
 `