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

const kinds = ({ kind, theme, shadow }) => {
  if (kind === "secondary") {
    return {
      background: theme.colors.primary.dark,
      color: theme.colors.text.primary
    }
  }

  if (kind === "warning") {
    return {
      background: theme.colors.warning,
      color: theme.colors.primary.contrastText,
      boxShadow: shadow && `rgba(255, 154, 0, 0.24) 0px 0px 0px, rgba(255, 145, 0, 0.56) 0px 4px 16px`
    }
  }

  if (kind === "danger") {
    return {
      background: theme.colors.danger,
      color: theme.colors.primary.contrastText,
      boxShadow: shadow && `rgba(245, 0, 87, 0.24) 0px 0px 0px, rgba(245, 0, 87, 0.56) 0px 4px 16px`
    }
  }

  // default is success
  return {
    background: theme.colors.success,
    color: theme.colors.primary.contrastText,
    boxShadow: shadow && `rgba(0, 191, 165, 0.24) 0px 0px 0px, rgba(0, 191, 165, 0.56) 0px 4px 16px`
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