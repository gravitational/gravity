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
import { K8sNodesProvider } from './dataProviders';
import k8sNodeGetters from './../../flux/k8sNodes/getters';
import { displayDate } from 'app/lib/dateUtils';
import { Wrap, JsonContent } from './items.jsx';
import {
  Table,
  Column,
  Cell,
  RowDetails,
  ToggableCell } from 'app/components/common/tables/table.jsx';

const HeartbeatCell = ({ rowIndex, data }) => {
  let date = new Date(data[rowIndex].condition.time);
  let text = displayDate(date);
  return (
    <Cell className="grv-site-k8s-nodes-heart">
      {text}
    </Cell>
  )
}
  
const NameCell = ({ rowIndex, expanded, data }) => {
  let { name, status } = data[rowIndex];
  let isExpanded = expanded[rowIndex] === true;

  return (
    <ToggableCell isExpanded={isExpanded} >
      <div>
        <div style={{fontSize: "14px"}}> {name}</div>
        <small>cpu: {status.capacity.cpu}, </small><small>ram: {status.capacity.memory}</small><br/>
        <small>os: {status.nodeInfo.osImage}</small><br/>
      </div>
    </ToggableCell>
  )
};

const StatusCell = ({ rowIndex, data }) => {
  let { condition } = data[rowIndex];
  return (
    <Cell className="grv-site-k8s-nodes-status">      
       <strong className={condition.icon}>{condition.message}</strong>      
    </Cell>
  );
};

const LabelCell = ({ rowIndex, expanded, data }) => {
  let { labels } = data[rowIndex];
  let $labels = [];
  let isExpanded = expanded[rowIndex];

  for(let key in labels){
   $labels.push( (
      <div key={key} className="grv-site-k8s-table-label">
        <div className="label">{ key + ":" + labels[key] }</div>
      </div>
    ));
  }

  return (
    <Wrap
      maxKids={3}
      className="grv-site-k8s-nodes-label"
      isExpanded={isExpanded} >
      {$labels}
    </Wrap>
  )
};

class NodePage extends React.Component {

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
    let { infoArray } = this.props;
    return (
      <div className="grv-site-k8s-nodes">
        <K8sNodesProvider />
        <Table
          className="grv-table-with-details grv-site-k8s-table"
          rowCount={infoArray.length}
          data={infoArray}
          onRowClick={this.onRowClick} >
          <RowDetails            
            content={<JsonContent colSpan={4} expanded={expanded} columnKey="nodeMap"/>}
          />
          <Column
            header={<Cell className="--col-name">Node</Cell> }
            cell={<NameCell expanded={expanded}/> }
          />
          <Column
            header={<Cell className="--col-status">Status</Cell> }
            cell={<StatusCell/> }
          />
          <Column
            header={<Cell>Labels</Cell> }
            cell={<LabelCell expanded={expanded}/> }
          />
          <Column
            header={<Cell>Heartbeat</Cell> }
            cell={<HeartbeatCell/> }
          />
      </Table>
    </div>
    )
  }
}

const mapStateToProps = () => ({
  infoArray: k8sNodeGetters.nodeInfos    
  }  
)

export default connect(mapStateToProps)(NodePage);
