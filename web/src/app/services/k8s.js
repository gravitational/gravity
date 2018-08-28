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

import api from './api';
import cfg from 'app/config';
import { formatPattern } from 'app/lib/patternUtils';
import { utils } from 'app/flux';

const k8s = {

  getNamespaces(siteId){
    let accountId = utils.getAccountId();
    let url = formatPattern(cfg.api.k8sNamespacePath, {siteId, accountId});
    return api.get(url).then( json => json.items );
  },

  saveConfigMap(siteId, namespace, name, data){
    let accountId = utils.getAccountId();
    let url = formatPattern(
      cfg.api.k8sConfigMapsByNamespacePath,
      {siteId, accountId, namespace, name}
    );

    return api.patch(url, data);
  },

  getConfigMaps(siteId){
    let accountId = utils.getAccountId();
    let url = formatPattern(cfg.api.k8sConfigMapsPath, {siteId, accountId});
    return api.get(url).then( json => json.items );
  },

  getNodes(siteId){
    let accountId = utils.getAccountId();
    let url = formatPattern(cfg.api.k8sNodesPath, {siteId, accountId});
    return api.get(url).then( json => json.items );
  },

  getJobs(siteId){
    let accountId = utils.getAccountId();
    let url = formatPattern(cfg.api.k8sJobsPath, {siteId, accountId});
    return api.get(url).then( json => json.items );
  },

  getPods(siteId, namespace){    
    let accountId = utils.getAccountId();
    let url = cfg.api.k8sPodsPath;
    if(namespace){
      url = cfg.api.k8sPodsByNamespacePath;
    }

    url = formatPattern(url, {siteId, accountId, namespace});
    return api.get(url).then( json => json.items );
  },

  getServices(siteId){
    let accountId = utils.getAccountId();
    let url = formatPattern(cfg.api.k8sServicesPath, {siteId, accountId});
    return api.get(url).then( json => json.items );
  },

  getDeployments(siteId){
    let accountId = utils.getAccountId();
    let url = formatPattern(cfg.api.k8sDelploymentsPath, {siteId, accountId});
    return api.get(url).then( json => json.items );
  },

  getDaemonSets(siteId){
    let accountId = utils.getAccountId();
    let url = formatPattern(cfg.api.k8sDaemonSetsPath, {siteId, accountId});
    return api.get(url).then( json => json.items );
  }
}

export default k8s;