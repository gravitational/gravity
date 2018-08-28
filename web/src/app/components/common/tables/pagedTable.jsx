/*
Copyright 2018 Gravitational, Inc.

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
import classnames from 'classnames';
import { Table } from './table.jsx';

var PagedTable = React.createClass({

  onPrev(){
    let { startFrom, pageSize } = this.state;

    startFrom = startFrom - pageSize;

    if( startFrom < 0){
      startFrom = 0;
    }

    this.setState({
      startFrom
    })

  },

  onNext(){
    let { data=[] } = this.props;
    let { startFrom, pageSize } = this.state;
    let newStartFrom = startFrom + pageSize;

    if( newStartFrom < data.length){
      newStartFrom = startFrom + pageSize;
      this.setState({
        startFrom: newStartFrom
      })
    }
  },

  getInitialState(){
    let { pageSize=7 } = this.props;
    return {
      startFrom: 0,
      pageSize
    }
  },

  componentWillReceiveProps(newProps){
    let newData = newProps.data || [];
    let oldData = this.props.data || [];
    // if data length changes, reset paging
    if(newData.length !== oldData.length){
      this.setState({startFrom: 0})
    }
  },

  render(){
    let { startFrom, pageSize } = this.state;
    let { data=[], tableClass='', className='' } = this.props;

    let endAt = 0;
    let totalRows = data.length;
    let pagedData = data;

    if (data.length > 0){
      endAt = startFrom + (pageSize > data.length ? data.length : pageSize);

      if(endAt > data.length){
        endAt = data.length;
      }

      pagedData = data.slice(startFrom, endAt);
    }

    let tableProps = {
      ...this.props,
      rowCount: pagedData.length,
      data: pagedData
    }

    let infoProps = {
      pageSize,
      startFrom,
      endAt,
      totalRows
    }

    return (
      <div className={className}>
        <div className={tableClass}>
          <Table {...tableProps} />
        </div>
        <PageInfo {...infoProps} onPrev={this.onPrev} onNext={this.onNext} />
      </div>
    )
  }
});

const PageInfo = (props) => {
  let {startFrom, endAt, totalRows, onPrev, onNext, pageSize} = props;

  let shouldBeDisplayed = totalRows > pageSize;

  if(!shouldBeDisplayed){
    return null;
  }

  let prevBtnClass = classnames('btn btn-white', {
    'disabled': startFrom === 0
  });

  let nextBtnClass = classnames('btn btn-white', {
    'disabled': endAt === totalRows
  });

  return (
    <div className="m-b-sm grv-table-paged-info">
      <span className="m-r-sm">
        <span className="text-muted">Showing </span>
        <span className="font-bold">{startFrom+1}</span>
        <span className="text-muted"> to </span>
        <span className="font-bold">{endAt}</span>
        <span className="text-muted"> of </span>
        <span className="font-bold">{totalRows}</span>
      </span>
      <div className="btn-group btn-group-sm">
        <a onClick={onPrev} className={prevBtnClass} type="button">Prev</a>
        <a onClick={onNext} className={nextBtnClass} type="button">Next</a>
      </div>
    </div>
  )
}

const EmptyIndicator = ({text}) => (
  <div className="grv-table-indicator-empty text-center text-muted"><span>{text}</span></div>
)

export default PagedTable;

export {
  PagedTable,
  EmptyIndicator
};
