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
import PropTypes from 'prop-types';
import TextEditor from 'oss-app/components/TextEditor';
import Dialog, { DialogFooter, DialogHeader, DialogTitle, DialogContent} from 'shared/components/Dialog';
import { ButtonSecondary } from 'shared/components';

class K8sResourceViewer extends React.Component {

  render() {
    const { json, onClose, title } = this.props;
    return (
      <Dialog
        dialogCss={dialogCss}
        disableEscapeKeyDown={false}
        onClose={onClose}
        open={true}
        >
        <DialogHeader>
          <DialogTitle caps={false} color="primary.contrastText">{title}</DialogTitle>
        </DialogHeader>
        <DialogContent my="0">
          <TextEditor readOnly={true} data={json} />
        </DialogContent>
        <DialogFooter>
          <ButtonSecondary onClick={onClose}>
            Close
          </ButtonSecondary>
        </DialogFooter>
      </Dialog>
    )
  }
}

K8sResourceViewer.propTypes = {
  json: PropTypes.string.isRequired,
  onClose: PropTypes.func.isRequired,
}

const dialogCss = () => `
  height: 80%
  width: calc(100% - 20%);
  max-width: 1400px;
`

export default K8sResourceViewer;
