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
import { Link } from 'react-router';
import reactor from 'app/reactor';
import { Table, Column, Cell, TextCell } from 'app/components/common/tables/table.jsx';
import { SiteOpProvider } from './../dataProviders';
import * as histActions from './../../flux/history/actions';
import histGetters from './../../flux/history/getters';
import Progress from './progress';

const ProgressCell = ({ data, rowIndex, ...props }) => {
  return (
    <Cell {...props}>
      <Progress {...data[rowIndex]} />
    </Cell>
  )
}

const LogLink = ({url, className, children}) => (
  <Link className={className} to={url}>
    {children}
  </Link>
)

const NameCell = ({ rowIndex, data, ...props }) => {
  let { logsUrl, displayType } = data[rowIndex];
  return (
    <Cell {...props}>
      <LogLink url={logsUrl}>
        <div className="grv-site-k8s-pods-name">{displayType}</div>
      </LogLink>
    </Cell>
  )
}



const ActionCell = ({ rowIndex, data, ...props }) => {
  let { logsUrl } = data[rowIndex];
  return (
    <Cell {...props}>
      <div className="pull-right">
        <LogLink url={logsUrl} className="btn btn-xs btn-outline btn-default m-l-xs">
          Logs
        </LogLink>
      </div>
    </Cell>
  )
};

const SiteHistory = React.createClass({

  mixins: [reactor.ReactMixin],

  componentDidMount(){
    histActions.init();
  },

  getDataBindings() {
    return {
      ops: histGetters.siteOps,
      model: histGetters.siteHistory
    }
  },

  render() {
    let {isInitialized } = this.state.model;
    if(!isInitialized){
      return null;
    }

    let data = this.state.ops;

    return (
      <div className="grv-site-history grv-page">
        <div>
          <div>
            <h3 className="grv-site-header-size m-b">Operations</h3>
          </div>
          <div className="grv-site-table">
            <Table data={data} rowCount={data.length}>
              <Column
                header={<Cell>Type</Cell> }
                cell={<NameCell/> }
              />
              <Column
                columnKey="created"
                header={<Cell>Started</Cell> }
                cell={<TextCell/> }
              />
              <Column
                header={<Cell>Status</Cell> }
                cell={ <ProgressCell/> }/>
              <Column
                cell={ <ActionCell/> }
              />
            </Table>
          </div>
        </div>
        <SiteOpProvider/>
      </div>
    )
  }
});

export default SiteHistory;
