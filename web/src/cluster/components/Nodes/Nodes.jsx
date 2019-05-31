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
import { useFluxStore } from 'app/components/nuclear';
import { withState } from 'shared/hooks';
import { getters } from 'app/cluster/flux/nodes';
import { fetchNodes } from 'app/cluster/flux/nodes/actions';
import { ButtonPrimary } from 'shared/components';
import NodeList from './NodeList';
import AddNodeDialog from './AddNodeDialog';
import DeleteNodeDialog from './DeleteNodeDialog';
import AjaxPoller from 'app/components/dataProviders'
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';

const POLLING_INTERVAL = 10000; // every 10 sec

export function Nodes({ nodes, onFetch }){
  // state
  const [ isAddNodeDialogOpen, setIsAddNodeDialogOpen ] = React.useState(false);
  const [ nodeToDelete, setNodeToDelete ] = React.useState(null);

  // actions
  const openAddDialog = () => setIsAddNodeDialogOpen(true);
  const closeAddDialog = () => setIsAddNodeDialogOpen(false);
  const openDeleteDialog = nodeToDelete => setNodeToDelete(nodeToDelete);
  const closeDeleteDialog = () => setNodeToDelete(null);

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>
          Nodes
        </FeatureHeaderTitle>
        <ButtonPrimary ml="auto" width="200px" onClick={openAddDialog}>
          Add Node
        </ButtonPrimary>
      </FeatureHeader>
      <NodeList onDelete={openDeleteDialog} nodes={nodes} />
      { isAddNodeDialogOpen && ( <AddNodeDialog onClose={closeAddDialog} /> )}
      { nodeToDelete && <DeleteNodeDialog node={nodeToDelete} onClose={closeDeleteDialog}/>}
      <AjaxPoller time={POLLING_INTERVAL} onFetch={onFetch} />
    </FeatureBox>
  )
}

const mapState = () => {
  const nodeStore = useFluxStore(getters.nodeStore);
  return {
    onFetch: fetchNodes,
    nodes: nodeStore.nodes
  }
}

export default withState(mapState)(Nodes);