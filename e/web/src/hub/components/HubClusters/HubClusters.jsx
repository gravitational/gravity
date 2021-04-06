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
import { values } from 'lodash';
import { useFluxStore } from 'oss-app/components/nuclear';
import CardEmpty from 'oss-app/components/CardEmpty';
import AjaxPoller from 'oss-app/components/AjaxPoller';
import { getters } from 'e-app/hub/flux/clusters';
import { refreshClusters } from 'e-app/hub/flux/actions';
import { withState } from 'shared/hooks';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from '../components/Layout';
import ClusterTileList from './ClusterTileList';
import HubClusterStore, { Provider, useStore } from './hubClusterStore';
import DisconnectDialog from './ClusterDisconnectDialog';

const POLL_INTERVAL = 4000; // every 4 sec

export function HubClusters(props){
  const { clusters, store, onRefresh } = props;
  const { setClusterToDisconnect } = store;
  const { clusterToDisconnect } = store.state;

  // ignore HUB cluster
  const joinedClusters = clusters.filter(c => !c.local);
  const isEmpty = joinedClusters.length === 0;

  useStore(store);

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          Clusters
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Provider value={store}>
        { isEmpty && <CardEmpty title="There are no joined clusters"/> }
        { !isEmpty && <ClusterTileList clusters={joinedClusters}/> }
        { clusterToDisconnect && (
            <DisconnectDialog
              cluster={clusterToDisconnect}
              onClose={ () => setClusterToDisconnect(null) }
            />
        )}
      </Provider>
      <AjaxPoller time={POLL_INTERVAL} onFetch={onRefresh} />
    </FeatureBox>
  )
}

function mapState(){
  const [ store ] = React.useState(() => {
    return new HubClusterStore()
  });

  const clusterStore = useFluxStore(getters.clusterStore);
  return {
    store,
    clusters: values(clusterStore.clusters),
    onRefresh:  refreshClusters
  }
}

export default withState(mapState)(HubClusters)