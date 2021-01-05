// oss imports
import connect from 'oss-app/lib/connect';
import UsersFeature from 'oss-app/modules/settings/features/userFeature';
import UsersPage from 'oss-app/modules/settings/components/users/main';

// local imports
import * as roleFlux from './../flux/roles';

const withRoles = Component => {
  return connect(() => ({ roles: roleFlux.getters.store }))(Component)
}

class EnterpriseUsersFeature extends UsersFeature {
  constructor(routes) {
    super(routes, withRoles(UsersPage))
  }
}

export default EnterpriseUsersFeature