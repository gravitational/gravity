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
import { connect } from 'app/components/nuclear';
import Poller from './../components/Poller';
import { fetchServices } from 'app/cluster/flux/k8sServices/actions';
import { getters } from 'app/cluster/flux/k8sServices';
import ServiceList from './ServiceList';

export function Services(props) {
  const { namespace, services, onFetch } = props;
  return (
    <React.Fragment>
      <ServiceList services={services} namespace={namespace} />
      <Poller namespace={namespace} onFetch={onFetch} />
    </React.Fragment>
  )
}

const subToStore = () => {
  return {
    services: getters.serviceInfoList
  }
}

const stateToProps = ({match}) => {
  const { namespace } = match.params;
  return {
    onFetch: fetchServices,
    namespace
  }
}

export default connect(subToStore, stateToProps)(Services);