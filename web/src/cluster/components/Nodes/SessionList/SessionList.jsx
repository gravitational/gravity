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
import { keyBy } from 'lodash';
import { Table, Column, Cell, TextCell } from 'shared/components/DataTable';
import { Text, ButtonPrimary } from 'shared/components';
import  * as Labels  from 'shared/components/Label';
import cfg from 'app/config';

const UserCell = ({ rowIndex, data}) => {
  const { parties } = data[rowIndex];
  const $parties = parties.map(({ user, remoteAddr }, index)  => {
    const text = `${user}/${remoteAddr}`;
    return (
      <Labels.Primary key={`${index}/${text}`} mb="1" mr="2">
        {`${user}/${remoteAddr}`}
      </Labels.Primary>
    )}
  )

  return (
    <Cell>
      {$parties}
    </Cell>
  );
}

export const ActionCell = ({ rowIndex, data}) => {
  const { sid } = data[rowIndex];
  const url = cfg.getConsoleSessionRoute({ sid})
  return (
    <Cell  style={{textAlign:"right"}}>
      <ButtonPrimary as="a" target="_blank"
        href={url}
        size="small" width="100px" children="join"
      />
    </Cell>
  )
}

const NodeCell = ({ nodes, rowIndex, data }) => {
  const { serverId, login  } = data[rowIndex];
  const server = nodes[serverId]

  let hostname = serverId;
  if(server) {
    hostname = server.hostname;
  }
  return (
    <Cell>
      <Text>{`${login}@${hostname}`}</Text>
    </Cell>
  )
};

const NodeProfileCell = ({ nodes, rowIndex, data }) => {
  const { serverId  } = data[rowIndex];
  const server = nodes[serverId]

  let displayRole = '';
  if(server) {
    displayRole = server.displayRole;
  }
  return (
    <Cell>
      <Text>{displayRole}</Text>
    </Cell>
  )
};

class SessionList extends React.Component {
  render() {
    const { sessions, nodes } = this.props;
    const keyedNodes = keyBy(nodes, 'id');
    return (
      <Table data={sessions} rowCount={sessions.length}>
        <Column
          header={<Cell>User / IP Address</Cell> }
          cell={<UserCell /> }
        />
        <Column
          header={<Cell>Login</Cell> }
          cell={<NodeCell nodes={keyedNodes}/> }
        />
        <Column
          header={<Cell>Node Profile</Cell> }
          cell={<NodeProfileCell nodes={keyedNodes}/> }
        />
        <Column
          columnKey="durationText"
          header={<Cell>Duration</Cell> }
          cell={<TextCell /> }
        />
        <Column
          header={<Cell/>}
          cell={ <ActionCell /> }
        />
      </Table>
    )
  }
}

export default SessionList;

