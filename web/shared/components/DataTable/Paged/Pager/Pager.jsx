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
import styled from 'styled-components'
import Icon, { CircleArrowLeft, CircleArrowRight } from 'shared/components/Icon';
import { Text } from 'shared/components';
import { borderRadius } from 'shared/system';

export default function Pager(props) {
  /* eslint-disable no-unused-vars */
  const { startFrom, endAt, data, totalRows, onPrev, onNext, pageSize, ...styles } = props;
  /* eslint-enable no-unused-vars */

  const shouldBeDisplayed = totalRows > pageSize;

  if(!shouldBeDisplayed){
    return null;
  }

  const isPrevDisabled = startFrom === 0;
  const isNextDisabled = endAt === totalRows;

  return (
    <StyledPager borderRadius="3" {...styles}>
      <Text typography="body2" color="primary.contrastText">
        SHOWING <strong>{startFrom+1}</strong> to <strong>{endAt}</strong> of <strong>{totalRows}</strong>
      </Text>
      <ButtonList>
        <button onClick={onPrev} title="Previous Page" disabled={isPrevDisabled}>
          <CircleArrowLeft fontSize="3" />
        </button>
        <button onClick={onNext} title="Next Page" disabled={isNextDisabled}>
          <CircleArrowRight fontSize="3" />
        </button>
      </ButtonList>
    </StyledPager>
  )
}

const ButtonList = styled.div`
  margin-left: auto;
`

const StyledPager = styled.nav`
  padding: 16px;
  display: flex;
  height: 24px;
  align-items: center;
  background: ${props => props.theme.colors.primary.light };
  button {
    background: none;
    border: none;
    border-radius: 200px;
    cursor: pointer;
    height: 24px;
    padding: 0;
    margin: 0 2px;
    min-width: 24px;
    outline: none;
    transition: all .3s;

    &:hover {
      background: ${props => props.theme.colors.primary.main };
      ${Icon} {
        opacity: 1;
      }
    }

    ${Icon} {
      opacity: .56;
      transition: all .3s;
    }
  }

  ${borderRadius}
`;
