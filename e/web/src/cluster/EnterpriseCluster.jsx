import React from 'react';

// oss imports
import Cluster from 'oss-app/cluster/components';
import { initCluster } from 'oss-app/cluster/flux/actions';
import FeatureDashboard from 'oss-app/cluster/features/featureDashboard';
import FeatureAccount from 'oss-app/cluster/features/featureAccount';
import FeatureNodes from 'oss-app/cluster/features/featureNodes';
import FeatureLogs from 'oss-app/cluster/features/featureLogs';
import FeatureMonitoring from 'oss-app/cluster/features/featureMonitoring';
import FeatureCertificate from 'oss-app/cluster/features/featureCertificate';
import FeatureAudit from 'oss-app/cluster/features/featureAudit';
import FeatureK8s from 'oss-app/cluster/features/featureK8s';
import 'oss-app/cluster/flux';

import { withState } from 'shared/hooks';
import FeatureLicense from './features/featureLicense';
import FeatureRoles from './features/featureRoles';
import FeatureAuthConnectors from './features/featureAuthConnectors';
import FeatureUsers from './features/featureUsers';
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
      new FeatureRoles(),
      new FeatureUsers(),
      new FeatureK8s(),
      new FeatureMonitoring(),
      new FeatureAuthConnectors(),
      new FeatureCertificate(),
      new FeatureLicense(),
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