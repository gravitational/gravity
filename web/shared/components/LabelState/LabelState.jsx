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
import { fade } from 'shared/theme/utils/colorManipulator';

const kinds = props => {
  // default is primary
  let kindStyles = {
    background: props.theme.colors.secondary.main,
    color: props.theme.colors.text.secondary.contrastText
  }

  if (props.kind === "secondary") {
    kindStyles = {
      background: props.theme.colors.primary.dark,
      color: props.theme.colors.text.primary
    }

    if(props.shadow) {
      kindStyles.boxShadow = `0 0 8px ${fade(props.theme.colors.primary.dark, .24)}, 0 4px 16px ${fade(props.theme.colors.primary.dark, .56)}`;
    }
  }

  if (props.kind === "warning") {
    kindStyles = {
      background: props.theme.colors.warning,
      color: props.theme.colors.primary.contrastText,
    }

    if(props.shadow) {
      kindStyles.boxShadow = `0 0 8px ${fade(props.theme.colors.warning, .24)}, 0 4px 16px ${fade(props.theme.colors.warning, .56)}`;
    }
  }

  if (props.kind === "danger") {
    kindStyles = {
      background: props.theme.colors.danger,
      color: props.theme.colors.primary.contrastText,
    }

    if(props.shadow) {
      kindStyles.boxShadow = `0 0 8px ${fade(props.theme.colors.danger, .24)}, 0 4px 16px ${fade(props.theme.colors.danger, .56)}`;
    }
  }

  if (props.kind === "success") {
    kindStyles = {
      background: props.theme.colors.success,
      color: props.theme.colors.primary.contrastText,
    }

    if(props.shadow) {
      kindStyles.boxShadow = `0 0 8px ${fade(props.theme.colors.success, .24)}, 0 4px 16px ${fade(props.theme.colors.success, .56)}`;
    }
  }

  // default is primary
  return kindStyles;
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