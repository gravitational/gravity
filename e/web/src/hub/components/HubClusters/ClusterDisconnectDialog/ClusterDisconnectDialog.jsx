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

import React from 'react'
import PropTypes from 'prop-types';
import * as Alerts from 'shared/components/Alert';
import { withState, useAttempt } from 'shared/hooks';
import { ButtonSecondary, ButtonPrimary, Text } from 'shared/components';
import Dialog, { DialogHeader, DialogTitle, DialogContent, DialogFooter} from 'shared/components/DialogConfirmation';
import { unlinkCluster } from 'e-app/hub/flux/actions';

export function ClusterDisconnectDialog(props){
  const { onClose, cluster, attempt, attemptActions, onDisconnect } = props;
  const isDisabled = attempt.isProcessing;
  const siteId = cluster.id;

  function onOk(){
    attemptActions
    .do(() => {
      return onDisconnect();
    })
    .then(() => onClose());
  }

  return (
    <Dialog disableEscapeKeyDown={isDisabled} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>UNLINK A CLUSTER</DialogTitle>
      </DialogHeader>
      <DialogContent maxWidth="600px">
        {attempt.isFailed && <Alerts.Danger children={attempt.message} />}
        <Text typography="paragraph" color="primary.contrastText">
          You are about to permanently disconnect{" "}
          <Text as="span" bold>
            {siteId}
          </Text>{" "}
          cluster from the Hub.
          <Text>
            It will continue to work as usual in standalone mode.
          </Text>
          <Text> This operation cannot be undone. Are you sure? </Text>
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary mr="3" disabled={isDisabled} onClick={onOk}>
          Disconnect
        </ButtonPrimary>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

ClusterDisconnectDialog.propTypes = {
  cluster: PropTypes.object.isRequired,
  attempt: PropTypes.object.isRequired,
  attemptActions: PropTypes.object.isRequired,
  onClose: PropTypes.func.isRequired,
  onDisconnect: PropTypes.func.isRequired,
}

function mapState(props) {
  const [ attempt, attemptActions ] = useAttempt();
  return {
    attempt,
    attemptActions,
    onDisconnect: () => unlinkCluster(props.cluster.id)
  }
}

export default withState(mapState)(ClusterDisconnectDialog);