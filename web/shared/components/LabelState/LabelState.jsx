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
import { fontSize, color, width, space } from 'shared/system';

const kinds = props => {
  if (props.kind === "secondary") {
    return {
      background: props.theme.colors.primary.dark,
      color: props.theme.colors.text.primary
    }
  }

  if (props.kind === "warning") {
    return {
      background: props.theme.colors.warning,
      color: props.theme.colors.primary.contrastText
    }
  }

  if (props.kind === "danger") {
    return {
      background: props.theme.colors.danger,
      color: props.theme.colors.primary.contrastText
    }
  }

  if (props.kind === "success") {
    return {
      background: props.theme.colors.success,
      color: props.theme.colors.primary.contrastText
    }
  }

  // default is primary
  return {
    background: props.theme.colors.secondary.main,
    color: props.theme.colors.text.secondary.contrastText
  }
}

const LabelState = styled.span`
  border-radius: 100px;
  font-weight: bold;
  outline: none;
  text-transform: uppercase;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
  ${fontSize}
  ${space}
  ${kinds}
  ${width}
  ${color}
  ${props => props.shadow && `
    box-shadow: rgba(0, 191, 165, 0.24) 0px 0px 0px, rgba(0, 191, 165, 0.56) 0px 4px 16px;
  `}
`
LabelState.defaultProps = {
  fontSize: 0,
  px: 3,
  color: 'light',
  fontWeight: 'bold',
  shadow: false
}

export default LabelState;
export const StateDanger = props => <LabelState kind="danger" {...props} />
export const StateInfo = props => <LabelState kind="secondary" {...props} />
export const StateWarning = props => <LabelState kind="warning" {...props} />
export const StateSuccess = props => <LabelState kind="success" {...props} />