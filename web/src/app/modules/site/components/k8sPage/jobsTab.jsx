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
import connect from 'app/lib/connect';
import { sortBy } from 'lodash';
import k8sGetters from './../../flux/k8s/getters';
import { K8sJobsProvider } from './dataProviders';
import { JsonContent } from './items.jsx';
import {
  Table,
  Column,
  Cell,
  TextCell,
  RowDetails,
  ToggableCell } from 'app/components/common/tables/table.jsx';
  
const NameCell = ({ rowIndex, expanded, data }) => {
  let { name } = data[rowIndex];
  let isExpanded = expanded[rowIndex] === true;
  return (
    <ToggableCell isExpanded={isExpanded} >
      <div style={{fontSize: "14px"}}>{name}</div>
    </ToggableCell>
  )
};

const DesiredCell = ({ rowIndex, data }) => {
  let { desired } = data[rowIndex];
  desired = desired || 'none';
  return (
    <Cell className="grv-site-k8s-jobs-desired">      
       {desired}
    </Cell>
  );
};

const getSucceededStatus = value => value ? <div className="text-success">Succeeded: {value}</div> : null;
const getFailedStatus = value => value ? <div className="text-danger">Failed: {value}</div> : null;
const getActiveStatus = value => value ? <div className="text-warning">Active: {value}</div> : null;

const StatusCell = ({ rowIndex, data }) => {
  let { statusSucceeded, statusFailed, statusActive } = data[rowIndex];  
  return (
    <Cell className="grv-site-k8s-jobs-status">
      <strong> { getSucceededStatus(statusSucceeded) }</strong>
      <strong> { getFailedStatus(statusFailed) }</strong>
      <strong> { getActiveStatus(statusActive) }</strong>      
    </Cell>
  );
};

class JobsTab extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      expanded: []
    };
  }
  
  onRowClick = index => {
    let { expanded } = this.state;
    expanded[index] = !expanded[index];
    this.setState({expanded});
  }
    
  render() {      
    let { expanded } = this.state;
    let { jobsArray, namespace } = this.props;
    jobsArray = jobsArray.filter( item => item.namespace === namespace );
    jobsArray = sortBy(jobsArray, ['created']).reverse();

    return (                
      <div className="grv-site-k8s-jobs">        
        <K8sJobsProvider/>
        <Table
          className="grv-table-with-details grv-site-k8s-table"
          rowCount={jobsArray.length}
          data={jobsArray}
          onRowClick={this.onRowClick} >
          <RowDetails            
            content={<JsonContent colSpan={4} expanded={expanded} columnKey="nodeMap"/>}
          />
          <Column
            header={<Cell className="--col-name">Name</Cell> }
            cell={<NameCell expanded={expanded}/> }
          />
          <Column
            header={<Cell>Desired</Cell> }
            cell={<DesiredCell/> }
          />     
          <Column
            header={<Cell>Status</Cell> }
            cell={<StatusCell/> }
          />               
          <Column
            columnKey="createdDisplay"          
            header={<Cell>Age</Cell> }
            cell={<TextCell/> }
          />               
        </Table>
    </div>
    )
  }
}

const mapStateToProps = () => ({
  jobsArray: k8sGetters.k8sJobs
  }  
)

export default connect(mapStateToProps)(JobsTab);
