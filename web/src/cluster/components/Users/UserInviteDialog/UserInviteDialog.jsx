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
import { Formik } from 'formik';
import { Input, Box, LabelInput, ButtonPrimary, ButtonSecondary } from 'shared/components';
import * as Alerts from 'shared/components/Alert';
import { useAttempt, withState } from 'shared/hooks';
import Dialog, { DialogHeader, DialogTitle, DialogContent, DialogFooter} from 'shared/components/Dialog';
import * as actions from 'app/cluster/flux/users/actions';
import Select from 'app/components/Select';
import CmdText from 'app/components/CmdText';

export function UserInviteDialog(props){
  const { roles, onClose, attempt, attemptActions, onCreateInvite } = props;
  const { isFailed, isProcessing, isSuccess } = attempt;

  const onSubmit = values => {
    const userRoles = values.userRoles.map( r => r.value );
    attemptActions.start();
    onCreateInvite(values.userName, userRoles)
      .done(userToken => {
        attemptActions.stop(userToken.url);
      })
      .fail(err => {
        attemptActions.error(err);
      });
  }

  const selectOptions = roles.map( r => ({
    value: r,
    label: r
  }))

  return (
    <Formik
      validate={validateForm}
      onSubmit={onSubmit}
      initialValues={{
        userRoles: [],
        userName: ''
      }}>
      {
        formikProps => (
        <Dialog
          dialogCss={dialogCss}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <Box width="500px">
            <DialogHeader>
              <DialogTitle> CREATE A USER INVITE LINK </DialogTitle>
            </DialogHeader>
            <DialogContent>
              {isFailed && ( <Alerts.Danger children={attempt.message}/> )}
              {isSuccess && <CmdText cmd={attempt.message}/>}
              {!isSuccess && renderInputFields({...formikProps, selectOptions})}
            </DialogContent>
            <DialogFooter>
              {renderButtons({isProcessing, isSuccess, onClose, onSave: formikProps.handleSubmit })}
            </DialogFooter>
          </Box>
        </Dialog>
    )}
  </Formik>
  );
}

function validateForm(values) {
  const errors = {};
  if (!values.userName) {
    errors.userName = ' is required';
  }

  if (values.userRoles.length === 0) {
    errors.userRoles = ' is required';
  }

  return errors;
}

function renderButtons({ isProcessing, isSuccess, onClose, onSave}) {
  if(isSuccess){
    return (
      <ButtonSecondary onClick={onClose}>
        Close
      </ButtonSecondary>
    )
  }

  return (
    <React.Fragment>
      <ButtonPrimary mr="3" disabled={isProcessing} onClick={onSave}>
        CREATE INVITE LINK
      </ButtonPrimary>
      <ButtonSecondary disabled={isProcessing} onClick={onClose}>
        Cancel
      </ButtonSecondary>
    </React.Fragment>
  )
}

function renderInputFields({ values, errors, setFieldValue, touched, handleChange, selectOptions}) {
  const userNameError = Boolean(errors.userName && touched.userName);
  const userRolesError = Boolean(errors.userRoles && touched.userRoles);
  return (
    <React.Fragment>
      <LabelInput hasError={userNameError}>
        User Name
        {userNameError && errors.userName}
      </LabelInput>
      <Input autoFocus autoComplete="off" value={values.userName} name="userName" onChange={handleChange}>
      </Input>
      <LabelInput hasError={userRolesError}>
        Assign a role
        {userRolesError && errors.userRolesError}
      </LabelInput>
      <Select
        maxMenuHeight="200"
        placeholder="Click to select a role"
        isSearchable
        isMulti
        isSimpleValue
        clearable={false}
        value={values.userRoles}
        onChange={ values => setFieldValue('userRoles', values)}
        options={selectOptions}
      />
    </React.Fragment>
  )
}

const dialogCss = () => `
  overflow-y: visible;
`
function mapState(){
  const [ attempt, attemptActions ] = useAttempt();
  return {
    onCreateInvite: actions.createInvite,
    attempt,
    attemptActions
  }
}

export default withState(mapState)(UserInviteDialog)