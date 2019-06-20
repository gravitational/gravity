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

import React from 'react'
import { Box, Text, LabelInput, ButtonPrimary, ButtonSecondary } from 'shared/components';
import * as Alerts from 'shared/components/Alert';
import { useAttempt, withState } from 'shared/hooks';
import Dialog, { DialogHeader, DialogTitle, DialogContent, DialogFooter} from 'shared/components/DialogConfirmation';
import * as actions from 'oss-app/cluster/flux/users/actions';
import { Formik } from 'formik';
import Select from 'oss-app/components/Select';

export function UserEditDialog(props){
  const { roles, user, onClose, attempt, attemptActions, onSave } = props;
  const { isFailed, isProcessing  } = attempt;
  const { userId } = user;

  const onSubmit = values => {
    const userRoles = values.userRoles.map( r => r.value );
    attemptActions
      .do(() =>  onSave(userId, userRoles))
      .then(() => {
        onClose();
      }
    )
  }

  const selectOptions = roles.map( r => ({
    value: r,
    label: r
  }))

  const selectInitialValue = user.roles.map( r => ({
    value: r,
    label: r
  }))

  return (
    <Formik
      validate={validateForm}
      onSubmit={onSubmit}
      initialValues={{
        userRoles: selectInitialValue,
      }}>
      {
        formikProps => (
        <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true} dialogCss={dialogCss}>
          <Box as="form" width="450px" onSubmit={formikProps.handleSubmit}>
            <DialogHeader>
              <DialogTitle>Edit User Role</DialogTitle>
              {isFailed && (
                <Alerts.Danger mt="4">
                  {attempt.message}
                </Alerts.Danger>
              )}
            </DialogHeader>
            <DialogContent>
              <Text mb="3" typography="paragraph" color="primary.contrastText">
                User: {userId}
              </Text>
              <Box>
                {renderInputFields({...formikProps, selectOptions})}
              </Box>
            </DialogContent>
            <DialogFooter>
              {renderButtons({isProcessing, onClose})}
            </DialogFooter>
          </Box>
        </Dialog>
    )}
  </Formik>
  );
}

function validateForm(values) {
  const errors = {};
  if (values.userRoles.length === 0) {
    errors.userRoles = ' is required';
  }

  return errors;
}

function renderButtons({ isProcessing, onClose}) {
  return (
    <React.Fragment>
      <ButtonPrimary mr="3" disabled={isProcessing} type="submit">
          SAVE Changes
      </ButtonPrimary>
      <ButtonSecondary disabled={isProcessing} onClick={onClose}>
        Cancel
      </ButtonSecondary>
    </React.Fragment>
  )
}

function renderInputFields({ values, errors, setFieldValue, touched, selectOptions}) {
  const userRolesError = Boolean(errors.userRoles && touched.userRoles);
  return (
    <React.Fragment>
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

function mapState(){
  const [ attempt, attemptActions ] = useAttempt();
  return {
    onSave: actions.saveUser,
    attempt,
    attemptActions
  }
}

const dialogCss = () => `
  overflow: visible;
`

export default withState(mapState)(UserEditDialog)