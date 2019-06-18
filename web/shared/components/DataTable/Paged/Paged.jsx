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
import { Table } from './../Table';
import Icon, { CircleArrowLeft, CircleArrowRight } from 'shared/components/Icon';
import Text from './../../Text';

function PagedTable(props){
  const { pageSize = 7, pagerPosition, data } = props;
  const [ startFrom, setFrom ] = React.useState(0);

  // set current page to 0 when data source length changes
  React.useEffect(() => {
    setFrom(0);
  }, [data.length]);

  function onPrev(){
    let prevPage = startFrom - pageSize;
    if(prevPage < 0){
      prevPage = 0;
    }
    setFrom(prevPage);
  }

  function onNext(){
    let nextPage = startFrom + pageSize;
    if(nextPage < data.length){
      nextPage = startFrom + pageSize;
      setFrom(nextPage);
    }
  }

  const totalRows = data.length;

  let endAt = 0;
  let pagedData = data;

  if (data.length > 0){
    endAt = startFrom + (pageSize > data.length ? data.length : pageSize);
    if(endAt > data.length){
      endAt = data.length;
    }

    pagedData = data.slice(startFrom, endAt);
  }

  const tableProps = {
    ...props,
    rowCount: pagedData.length,
    data: pagedData
  }

  const infoProps = {
    pageSize,
    startFrom,
    endAt,
    totalRows
  }

  let $pager = null;
  if(totalRows > pageSize) {
    $pager = <PageInfo {...infoProps} onPrev={onPrev} onNext={onNext} />;
  }

  const showTopPager = !pagerPosition || pagerPosition === 'top';
  const showBottomPager = !pagerPosition || pagerPosition === 'bottom';
  return (
    <div>
      { showTopPager && $pager}
      <Table {...tableProps} />
      { showBottomPager && $pager}
    </div>
  )
}


const PageInfo = props => {
  const {startFrom, endAt, totalRows, onPrev, onNext, pageSize} = props;
  const shouldBeDisplayed = totalRows > pageSize;

  if(!shouldBeDisplayed){
    return null;
  }

  const isPrevDisabled = startFrom === 0;
  const isNextDisabled = endAt === totalRows;

  return (
    <Pager>
      <Text typography="subtitle2" color="primary.contrastText" fontWeight="light">
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
    </Pager>
  )
}

export default PagedTable;

const ButtonList = styled.div`
  margin-left: auto;
`

const Pager = styled.nav`
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
`;
