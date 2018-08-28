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
import serviceGetters from './../../flux/k8sServices/getters';
import { K8sServiceProvider } from './dataProviders';
import {
  Table,
  Column,
  Cell,
  TextCell,
  RowDetails,
  ToggableCell } from 'app/components/common/tables/table.jsx';

import { JsonContent } from './items.jsx';

const NameCell = ({ rowIndex, expanded, data }) => {
  let { serviceName } = data[rowIndex];
  let isExpanded = expanded[rowIndex] === true;
  return (
    <ToggableCell isExpanded={isExpanded} >
      <div style={{fontSize: "14px"}}>{serviceName}</div>
    </ToggableCell>
  )
};

const PortCell = ({ rowIndex, data }) => {
  let service = data[rowIndex];

  let $ports = service.ports.map(( text, index ) => {
    let [port1, port2] = text.split('/');
    return (
      <div key={index} className="grv-site-k8s-table-label">
        <div className="label">
          {port1}<li className="fa fa-long-arrow-right m-r-xs m-l-xs"> </li>
          {port2}
        </div>
      </div>
    );
  });

  return (<Cell>{$ports}</Cell>);
};

const LabelCell = ({ rowIndex, data }) => {
  let service = data[rowIndex];
  let $labelItems = service.labels.map( (text, key) => (
    <div key={key} className="grv-site-k8s-table-label">
      <div className="label">{text}</div>
    </div>
    )
 );

  return (<Cell> {$labelItems} </Cell>);
};

class ServicesPage extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      expanded: []
    }    
  }
  
  onRowClick = index => {
    let { expanded } = this.state;
    expanded[index] = !expanded[index];
    this.setState({expanded});
  }
  
  render() {
    let { serviceInfos } = this.props;
    let { expanded } = this.state;
    let { namespace } = this.props;
    let filtered = serviceInfos.filter( item => item.serviceNamespace === namespace );

    return (
      <div className="grv-site-k8s-services">        
        <K8sServiceProvider />
        <Table
          className="grv-table-with-details grv-site-k8s-table"
          rowCount={filtered.length}
          data={filtered}
          onRowClick={this.onRowClick} >
          <RowDetails            
            content={<JsonContent colSpan={4} expanded={expanded} columnKey="serviceMap"/>}
          />
          <Column
            header={<Cell className="--col-name">Name</Cell> }
            cell={<NameCell expanded={expanded} /> }
          />
          <Column
            columnKey="clusterIp"
            header={<Cell className="--col-cluster">Cluster</Cell> }
            cell={<TextCell/> }
          />
          <Column
            header={<Cell className="--col-port">Ports</Cell> }
            cell={<PortCell/> }
          />
          <Column
            header={<Cell className="--col-label">Labels</Cell> }
            cell={<LabelCell/> }
          />
        </Table>
      </div>
    )
  }
}

const mapStateToProps = () => ({
  serviceInfos: serviceGetters.serviceInfoList  
})  

export default connect(mapStateToProps)(ServicesPage);