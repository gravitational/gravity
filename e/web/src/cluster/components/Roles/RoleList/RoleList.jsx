/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Table, Cell, Column } from 'shared/components/DataTable';
import { RoleNameCell, ActionCell, } from './RoleListCells';

const RoleList = ({ roles, onEdit, onDelete }) => {
  roles = roles || [];
  return (
    <Table data={roles}>
      <Column
        header={<Cell>Name</Cell> }
        cell={<RoleNameCell />}
        />
      <Column
        header={<Cell style={{textAlign: "right"}}>Actions</Cell>}
        cell={
          <ActionCell
            onEdit={onEdit}
            onDelete={onDelete} />
        }
      />
    </Table>
  )
};

export default RoleList;