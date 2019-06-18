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

import styled from 'styled-components';
import defaultTheme from './../../theme';
import { fontSize, space } from 'styled-system'

const values  = {
  fontSize: 1,
  pl: 10,
  pr: 5,
}

const fromTheme = ({ theme = defaultTheme }) => {
  values.theme = theme;
  return {
    ...fontSize(values),
    ...space(values),
    fontWeight: theme.bold,
    background: theme.colors.primary.main,
    color: theme.colors.text.primary,
    "&:active, &.active": {
      background: theme.colors.primary.light,
      borderLeftColor: theme.colors.accent,
      color: theme.colors.primary.contrastText
    },
    "&:hover": {
      background: theme.colors.primary.light,
    }
  }
}

const SideNavItem = styled.div`
  min-height: 72px;
  align-items: center;
  border: none;
  border-left: 4px solid transparent;
  box-sizing: border-box;
  cursor: pointer;
  display: flex;
  justify-content: flex-start;
  outline: none;
  text-decoration: none;
  width: 100%;
  ${fromTheme}
`;

SideNavItem.displayName = 'SideNavItem';

export default SideNavItem;
