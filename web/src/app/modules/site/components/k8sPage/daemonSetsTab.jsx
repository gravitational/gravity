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
import { K8sDaemonSetsProvider } from './dataProviders';
import { JsonContent } from './items.jsx';
import {
  Table,
  TextCell,
  Column,
  Cell,
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

class DaemonSetsTab extends React.Component {

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
    let { daemonsArray, namespace } = this.props;
    daemonsArray = daemonsArray.filter( item => item.namespace === namespace );
    daemonsArray = sortBy(daemonsArray, ['created']).reverse();
    return (                
      <div className="grv-site-k8s-daemonsets">        
        <K8sDaemonSetsProvider/>
        <Table
          className="grv-table-with-details grv-site-k8s-table"
          rowCount={daemonsArray.length}
          data={daemonsArray}
          onRowClick={this.onRowClick} >
          <RowDetails            
            content={<JsonContent colSpan={6} expanded={expanded} columnKey="nodeMap"/>}
          />
          <Column
            header={<Cell className="--col-name">Name</Cell> }
            cell={<NameCell expanded={expanded}/> }
          />
          <Column
            columnKey="statusDesiredNumberScheduled"
            header={<Cell>Desired</Cell> }
            cell={<TextCell/> }
          />
          <Column
            columnKey="statusCurrentNumberScheduled"
            header={<Cell>Current</Cell> }
            cell={<TextCell/> }
          />
          <Column
            columnKey="statusNumberReady"
            header={<Cell className="--col-ready">Ready</Cell> }
            cell={<TextCell/> }
          />
          <Column
            columnKey="statusNumberMisscheduled"
            header={<Cell>Misscheduled</Cell> }
            cell={<TextCell/> }
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
  daemonsArray: k8sGetters.k8sDaemonSets
  }  
)

export default connect(mapStateToProps)(DaemonSetsTab);
