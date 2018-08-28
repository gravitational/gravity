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
import { Table, Column, Cell } from 'app/components/common/tables/table.jsx';
import cfg from 'app/config';
import moment from 'moment';
import Button from 'app/components/common/button';
import Layout from 'app/components/common/layout';
import classnames from 'classnames';
import { UserStatusEnum } from 'app/services/enums'
import Box from 'app/components/common/boxes/box';
import { sortBy } from 'lodash'

const A = ({children, ...props}) => (
  <a href="" {...props} onClick={A.onClick(props)} >{children}</a>
);

A.onClick = props => e => {
  e.preventDefault();
  if (!props.disabled) {
      props.onClick();
  }
}

const getRoleDisplayName = (rName, roleLabels) => {
  let rlabel = roleLabels.find(rl => rl.value === rName);

  if (rlabel) {
    return rlabel.label;
  }

  return rName;
};

const RoleLabel = ({ name }) => (
  <label title={name} className="grv-settings-users-role-label m-r-xs">
    {name}
  </label>
)

const StatusCell = ({ rowIndex, data, ...props }) => {
  let text = 'unknown:';
  let {status, created} = data[rowIndex];
  let createdDate = created ? moment(created).format(cfg.dateFormat) : 'unknown';

  let className = classnames('text-muted m-r-sm', {
      'grv-settings-users-status-active': status === UserStatusEnum.ACTIVE,
      'grv-settings-users-status-invited': status === UserStatusEnum.INVITED,
  })

  if(status === UserStatusEnum.ACTIVE){
    text = 'joined on:'
  }else if(status === UserStatusEnum.INVITED){
    text = 'invited on:'
  }

  return (
    <Cell {...props}>
      <Layout.Flex dir="row" style={{ minWidth: '150px' }}>
        <small className={className}>{text} </small>
        <span>{createdDate}</span>
      </Layout.Flex>
    </Cell>
  )
};

const UserIdCell = ({ rowIndex, onClick, data, ...props }) => {
  let { status, userId } = data[rowIndex];
  return (
    <Cell {...props}>
      <div>
        { status === UserStatusEnum.ACTIVE &&  <a style={{float: "left"}} href="#" className="" onClick={ () => onClick(userId) }>{userId}</a> }
        { status !== UserStatusEnum.ACTIVE &&  <span style={{float: "left"}}>{userId}</span> }
      </div>
    </Cell>
  )
}

const RoleCell = ({ roleLabels, rowIndex, data, ...props }) => {
  let { roles } = data[rowIndex];

  let roleDisplayNames = roles.map(name =>
    getRoleDisplayName(name, roleLabels));

  let $roles = roleDisplayNames.map((r, index) =>
    <RoleLabel key={index} name={r} />);

  let $content = (
    <small className="text-muted">no assigned roles </small>
  );

  if ($roles.length > 0) {
    $content =  (
      <Layout.Flex dir="row" align="center">
        <small className="m-r-xs m-b-sm text-muted">
          roles:
        </small>
        <Layout.Flex dir="row" style={{ flexWrap: "wrap" }}>
          {$roles}
        </Layout.Flex>
      </Layout.Flex>
      )
  }

  return (
    <Cell {...props}>
      {$content}
    </Cell>
  )
}

const ButtonCell = ({ rowIndex, onReset, onEdit, onReInvite, onDelete, data, ...props }) => {
  let { userId, owner, status } = data[rowIndex];

  let $actions = null;
  let deleteBtnClass = owner ? 'disabled' : '';

  if (status === UserStatusEnum.INVITED) {
    $actions = [
      <li key="0">
        <A onClick={ () => onReInvite(userId) }>
          <i className="fa fa-repeat m-r-xs"></i>
          <span>Renew invitation...</span>
        </A>
      </li>,
      <li key="1" className="divider"></li>,
      <li key="2">
        <A onClick={ () => onDelete(userId) }>
          <i className="fa fa-trash m-r-xs"></i>
          <span>Revoke invitation...</span>
        </A>
      </li>,
    ]
  } else {
    $actions = [
      <li key="0">
        <A onClick={ () => onEdit(userId) }>
          <i className="fa fa-pencil m-r-xs"></i>
          <span>Edit</span>
        </A>
      </li>,
      <li key="1">
        <A onClick={ () => onReset(userId) }>
          <i className="fa fa-repeat m-r-xs"></i>
          <span>Password Reset...</span>
        </A>
      </li>,
      <li key="2" className="divider"></li>,
      <li key="3" className={deleteBtnClass}>
        <A disabled={owner} onClick={ () => onDelete(userId) }>
          <i className="fa fa-trash m-r-xs"></i>
          <span>Delete...</span>
        </A>
      </li>
    ]
  }

  return (
    <Cell {...props}>
      <div className="btn-group pull-right">
        <button type="button" className="btn btn-default btn-sm dropdown-toggle" data-toggle="dropdown" aria-haspopup="trufe" aria-expanded="false">
          <span className="m-r-xs">Actions</span>
          <span className="caret" />
        </button>
        <ul className="dropdown-menu dropdown-menu-right pull-right">
          {$actions}
        </ul>
      </div>
    </Cell>
  )
}

const UserList = React.createClass({
  render() {
    let {
      users,
      roleLabels,
      onReset,
      onAdd,
      onEdit,
      onDelete,
      onReInvite
    } = this.props;

    users = sortBy(users, u => u.created).reverse();

    return (
      <Box className="grv-settings-users-list">
        <Box.Header>
          <h3>Users</h3>
          <div className="text-right" style={UserList.headerStyle}>
            <Button
              className="btn-sm btn-default m-t-n-xs"
              isDisabled={false}
              onClick={onAdd}>
              <i className="fa fa-plus m-r-xs"/>
              New User
            </Button>
          </div>
        </Box.Header>
        <div className="grv-settings-users-table m-t-n-sm">
          <Table data={users} rowCount={users.length}>
            <Column className="--name"
              cell={<UserIdCell onClick={onEdit}/>}
            />
            <Column
              columnKey="isAdmin"
              cell={<RoleCell roleLabels={roleLabels}/> }
            />
            <Column
              cell={<StatusCell /> }
            />
            <Column
              cell={<ButtonCell
                onReInvite={onReInvite}
                onReset={onReset}
                onEdit={onEdit}
                onDelete={onDelete} />}
            />
          </Table>
        </div>
      </Box>
    )
  }
});

UserList.headerStyle = {
  flex: "1",
  height: "20px",
  marginTop: "-3px"
}

export default UserList;

