import React from 'react';
import { useFluxStore } from 'oss-app/components/nuclear';
import Users from 'oss-app/cluster/components/Users';
import { getters } from 'e-app/cluster/flux/roles';

export default function EnterpriseUsers(props) {
  const roles = useFluxStore(getters.store);
  return <Users roles={roles} {...props} />
}