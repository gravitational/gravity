/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import styled from 'styled-components';
import { StatusEnum } from 'oss-app/services/clusters';
import React from 'react';
import { Box } from 'shared/components';

export default function Status({status, ...styles}){
  return (
    <StyledStatus title={status} status={status} {...styles} />
  )
}

const StyledStatus = styled(Box)`
  width: 6px;
  height: 6px;
  border-radius: 50%;
  ${({ status, theme }) => {
    switch(status){
      case StatusEnum.OFFLINE:
        return {
          backgroundColor: theme.colors.grey[300],
          boxShadow: `0px 0px 12px 4px ${theme.colors.grey[300]}`
        }
      case StatusEnum.READY:
        return {
          backgroundColor: theme.colors.success,
          boxShadow: `0px 0px 12px 4px ${theme.colors.success}`
        }
      case StatusEnum.ERROR:
        return {
          backgroundColor: theme.colors.danger,
          boxShadow: `0px 0px 12px 4px ${theme.colors.danger}`
        }
      default:
        return {
          backgroundColor: theme.colors.warning,
          boxShadow: `0px 0px 12px 4px ${theme.colors.warning}`
        }
      }
    }
  }
`