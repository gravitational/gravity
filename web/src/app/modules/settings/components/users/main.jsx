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
import { SystemRoleEnum } from 'app/services/enums';
import usrGetters from './../../flux/users/getters';
import * as userActions from './../../flux/users/actions';
import UserList from './userList';
import AddEditUser from './addEditUser';
import DeleteDialog from './deleteUserDialog.jsx';
import ReinviteDialog from './reinviteUserDialog.jsx';
import ResetUserDialog from './resetUserDialog';

export class UsersPage extends React.Component {

  componentWillUnmount() {
    userActions.clear();
  }

  render() {
    const {
      store,
      roles,
      resetUserAttempt,
      deleteUserAttempt,
      saveUserAttempt,
      inviteUserAttempt } = this.props;

    const { selectedUser, userToReset, userToDelete, userToReinvite } = store;
    const roleLabels = !roles ? [SystemRoleEnum.TELE_ADMIN] : roles.getRoleNames();
    return (
      <div className="full-width">
        <UserList
          onReset={userActions.openResetUserDialog}
          onEdit={userActions.openEditUserDialog}
          onReInvite={userActions.openResendInviteDialog}
          onDelete={userActions.openDeleteUserDialog}
          onAdd={userActions.addUser}
          roleLabels={roleLabels}
          users={store.users} />
        { selectedUser &&
          <AddEditUser
            user={selectedUser}
            roleLabels={roleLabels}
            saveAttempt={saveUserAttempt}
            inviteAttempt={inviteUserAttempt}
            onSave={userActions.saveUser}
            onInvite={userActions.createInvite}
            onCancel={userActions.cancelAddEditUser}
            />
        }
        <ResetUserDialog
          userId={userToReset}
          onContinue={userActions.resetUser}
          onCancel={userActions.closeResetUserDialog}
          attempt={resetUserAttempt}/>
        <ReinviteDialog
          attemp={inviteUserAttempt}
          onCancel={userActions.closeResendInviteDialog}
          onContinue={userActions.reInviteUserUser}
          userId={userToReinvite} />
        <DeleteDialog
          attemp={deleteUserAttempt}
          onCancel={userActions.closeDeleteUserDialog}
          onContinue={userActions.deleteUser}
          userId={userToDelete} />
      </div>
    );
  }
}

function mapFluxToProps() {
  return {
    deleteUserAttempt: usrGetters.deleteUserAttempt,
    saveUserAttempt: usrGetters.saveUserAttempt,
    inviteUserAttempt: usrGetters.inviteUserAttempt,
    resetUserAttempt: usrGetters.resetUserAttempt,
    store: usrGetters.userStore
  }
}

export default connect(mapFluxToProps)(UsersPage);