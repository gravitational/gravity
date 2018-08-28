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

/**
* Sort indicator used by SortHeaderCell
*/
const SortTypes = {
  ASC: 'ASC',
  DESC: 'DESC'
};

const SortIndicator = ({sortDir})=>{
  let cls = 'grv-table-indicator-sort fa fa-sort'
  if(sortDir === SortTypes.DESC){
    cls += '-desc'
  }

  if(sortDir === SortTypes.ASC){
    cls += '-asc'
  }

  return <i className={cls}></i>;
};

/**
* Default Cell
*/
const GrvTableCell = ({ children, className='', isHeader, colSpan, rowSpan, style } ) => {
  className = 'grv-table-cell ' + className;
  return isHeader ? <th className={className}>{children}</th> : <td style={style} className={className} rowSpan={rowSpan} colSpan={colSpan}>{children}</td>;
}

const GrvTableTextCell = ({rowIndex, data, columnKey, ...props}) => (
  <GrvTableCell {...props}>
    {data[rowIndex][columnKey]}
  </GrvTableCell>
);

const ToggableCell = ({isExpanded, className, ...props}) => {
  let iconClass = 'grv-table-indicator-expand fa ';
  iconClass += isExpanded ? 'fa-chevron-down' : 'fa-chevron-right';
  return (
    <GrvTableCell className={className}>
      <div style={{display: "flex", alignItems: "baseline"}}>
        <li className={iconClass} />
        {props.children}
      </div>
    </GrvTableCell>
  )
}

/**
* Sort Header Cell
*/
var SortHeaderCell = React.createClass({
  render() {
    var { sortDir, title, ...props } = this.props;

    return (
      <GrvTableCell {...props}>
        <a onClick={this.onSortChange}>
          {title}
        </a>
        <SortIndicator sortDir={sortDir}/>
      </GrvTableCell>
    );
  },

  onSortChange(e) {
    e.preventDefault();
    if(this.props.onSortChange) {
      // default
      let newDir = SortTypes.DESC;
      if(this.props.sortDir){
        newDir = this.props.sortDir === SortTypes.DESC ? SortTypes.ASC : SortTypes.DESC;
      }
      this.props.onSortChange(this.props.columnKey, newDir);
    }
  }
});

var RowContent = props => <tr {...props} />;

/**
* Table
*/
var GrvTable = React.createClass({

  onRowClick(rowIndex){    
    if(this.props.onRowClick){
      this.props.onRowClick(rowIndex);
    }
  },

  renderHeader(children){
    let cells = children.map((item, key)=>{
      let cellProps = {
        key,
        isHeader: true,
        ...item.props
      };

      return cloneWithProps(item.props.header, cellProps);
    })

    return <thead className="grv-table-header"><tr>{cells}</tr></thead>
  },

  renderBody(columnCells, rowDetails){
    let { rowCount, data } = this.props;
    let rows = [];
    for(var i = 0; i < rowCount; i ++){
      let cells = columnCells.map((item, key)=>{
        let cellProps = {
          key,
          data,
          rowIndex: i,
          isHeader: false,
          ...item.props,
        };

        return cloneWithProps( item.props.cell, cellProps );
      })

      rows.push(<RowContent onClick={this.onRowClick.bind(this, i)} key={i}>{cells}</RowContent>);

      if(rowDetails){
        let { content } = rowDetails.props;
        let detailsKey = 'r-d' + i;
        let rowProps = {
          key: detailsKey,
          rowIndex: i,
          data,
        }

        let clonedContent = cloneWithProps(content, rowProps);

        rows.push(clonedContent);
      }
    }

    return <tbody>{rows}</tbody>;
  },

  render() {
    let columnCells = [];
    let rowDetails = null;
    var tableClass = this.props.className || '';

    tableClass = 'table grv-table ' + tableClass;

    React.Children.forEach(this.props.children, (child) => {
      if (child == null) {
        return;
      }

      if(child.type.displayName === 'GrvTableColumn'){
        columnCells.push(child);
      }

      if(child.type.displayName === 'GrvTableRowDetails'){
        rowDetails = child;
      }
    });

    return (
      <table className={tableClass} onClick={this.onClick}>
        {this.renderHeader(columnCells)}
        {this.renderBody(columnCells, rowDetails)}
      </table>
    );
  }
})

var GrvTableColumn = React.createClass({
  render() {
    throw new Error('Component <GrvTableColumn /> should never render');
  }
})

/**
* Row details
*/
var GrvTableRowDetails = React.createClass({
  render() {
    throw new Error('Component <GrvTableRowDetails /> should never render');
  }
});


const cloneWithProps = (element, props) => {
  let content = null;
  if (React.isValidElement(element)) {
     content = React.cloneElement(element, props);
   } else if (typeof cell === 'function') {
     content = element(props);
   }

  return content;
}

export default GrvTable;

export {
  GrvTableRowDetails as RowDetails,
  GrvTableColumn as Column,
  GrvTable as Table,
  GrvTableCell as Cell,
  GrvTableTextCell as TextCell,
  ToggableCell,
  RowContent,
  SortHeaderCell,
  SortIndicator,
  SortTypes
};
