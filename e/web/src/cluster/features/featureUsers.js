// oss imports
import FeatureUsers, { makeNavItem } from 'oss-app/cluster/features/featureUsers';
import withFeature from 'oss-app/components/withFeature';
import EnterpiseUsers from 'e-app/cluster/components/Users';

class EnterpriseUsersFeature extends FeatureUsers {
  constructor() {
    super()
    this.Component = withFeature(this)(EnterpiseUsers);
  }
}

export default EnterpriseUsersFeature;

export {
  makeNavItem
}