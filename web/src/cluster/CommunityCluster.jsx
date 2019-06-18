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

// oss imports
import Cluster from 'oss-app/cluster/components';
import { initCluster } from 'oss-app/cluster/flux/actions';
import FeatureDashboard from 'oss-app/cluster/features/featureDashboard';
import FeatureAccount from 'oss-app/cluster/features/featureAccount';
import FeatureNodes from 'oss-app/cluster/features/featureNodes';
import FeatureLogs from 'oss-app/cluster/features/featureLogs';
import FeatureUsers from 'oss-app/cluster/features/featureUsers';
import FeatureMonitoring from 'oss-app/cluster/features/featureMonitoring';
import FeatureCertificate from 'oss-app/cluster/features/featureCertificate';
import FeatureAudit from 'oss-app/cluster/features/featureAudit';
import FeatureK8s from 'oss-app/cluster/features/featureK8s';
import { withState } from 'shared/hooks';
import './flux';

function mapState(props){
  const { siteId } = props.match.params;
  const [ features ] = React.useState(() => {
    return [
      new FeatureDashboard(),
      new FeatureAccount(),
      new FeatureNodes(),
      new FeatureLogs(),
      new FeatureAudit(),
      new FeatureUsers(),
      new FeatureK8s(),
      new FeatureMonitoring(),
      new FeatureCertificate(),
    ]
  })

  function onInit(){
    return initCluster(siteId, features);
  }

  return {
    features,
    onInit,
  }
}

export default withState(mapState)(Cluster);