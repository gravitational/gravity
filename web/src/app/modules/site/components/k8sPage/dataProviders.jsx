/*
Copyright 2018 Gravitational, Inc.

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
import AjaxPoller from 'app/components/dataProviders'

import { fetchJobs, fetchDaemonSets, fetchDeployments } from './../../flux/k8s/actions';
import { fetchNodes } from './../../flux/k8sNodes/actions';
import { fetchPods } from './../../flux/k8sPods/actions';
import { fetchServices } from './../../flux/k8sServices/actions';

const POLL_INTERVAL = 10000; // 10 sec

export const K8sNodesProvider = () => (
  <AjaxPoller
    time={POLL_INTERVAL}
    onFetch={fetchNodes} />
)  

export const K8sJobsProvider = () => (
  <AjaxPoller
    time={POLL_INTERVAL}
    onFetch={fetchJobs} />
)  

export const K8sServiceProvider = () => (
  <AjaxPoller
    time={POLL_INTERVAL}
    onFetch={fetchServices}      
  />
)

export const K8sPodsProvider = () => (
  <AjaxPoller
    time={POLL_INTERVAL}
    onFetch={fetchPods}      
  />
)

export const K8sDaemonSetsProvider = () => (
  <AjaxPoller
    time={POLL_INTERVAL}
    onFetch={fetchDaemonSets}      
  />
)

export const K8sDeploymentsProvider = () => (
  <AjaxPoller
    time={POLL_INTERVAL}
    onFetch={fetchDeployments}      
  />
)
