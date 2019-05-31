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

import $ from 'jQuery';
import { at, map } from 'lodash';
import api from 'app/services/api';
import cfg from 'app/config';
import appService, { makeApplication } from 'app/services/applications';
import opService, { OpTypeEnum } from 'app/services/operations';
import makeFlavors from './makeFlavors';
import makeAgentServers from './makeAgentServer';

const service = {

  fetchAgentReport({ siteId, opId }){
    return api.get(cfg.getOperationAgentUrl(siteId, opId))
      .then(data => {
        return makeAgentServers(data);
      });
  },

  verifyOnPrem(request){
    const { siteId, opId } = request;
    return api.post(cfg.operationPrecheckPath(siteId, opId), request);
  },

  startInstall(request){
    const { siteId, opId } = request;
    return api.post(cfg.getOperationStartUrl(siteId, opId), request);
  },

  fetchClusterDetails(siteId){
    return $.when(
      // fetch operation
      opService.fetchOps(siteId),
      // fetch cluster app
      service.fetchAppByClusterId(siteId),
      // fetch flavors
      api.get(cfg.getSiteFlavorsUrl(siteId))
    )
      .then((...responses)=> {
        const [operations, app, flavorsJson ] = responses;
        const operation = operations.find(o => o.type === OpTypeEnum.OPERATION_INSTALL);
        const flavors = makeFlavors(flavorsJson, app, operation)
        return {
          app,
          flavors,
          operation
        }
      }
    )
  },

  createCluster(request){
    const url = cfg.getSiteUrl({});
    return service.verifyClusterName(request.domain_name)
      .then(() => {
         return api.post(url, request).then(json => json.site_domain);
      })
  },

  setDeploymentType(license, app_package){
    const request = {
      license,
      app_package
    }

    return api.post(cfg.api.licenseValidationPath, request)
      .then(() => {
        return license;
      });
  },

  verifyClusterName(name){
    return api.get(cfg.getCheckDomainNameUrl(name)).then(data => {
      data = data || [];
      if(data.length > 0){
        return $.Deferred().reject(new Error(`Cluster "${name}" already exists`))
      }
    })
  },

  verifyAwsKeys({ packageId, provider, accessKey, secretKey, sessionToken }){
    const request = {
      provider,
      variables: {
        access_key: accessKey,
        secret_key: secretKey,
        session_token: sessionToken,
      },
      application: packageId
    };

    return api.post(cfg.api.providerPath, request)
      .then(data => {
        return map(data.aws.regions, makeRegion);
      })
  },

  fetchApp(...params){
    return appService.fetchApplication(...params);
  },

  fetchAppByClusterId(siteId){
    return api.get(cfg.getSiteUrl({siteId, shallow: false}))
      .then(json => makeApplication(json.app))
  }
}

export function makeRegion(json){
  const [ name, vpcsJson, keyPairsJson ] = at(json, ['name', 'vpcs', 'key_pairs']);
  const vpcs = map(vpcsJson, makeVpc)
  const keyPairs = map(keyPairsJson, makeKeyPair);
  return {
    name,
    label: name,
    vpcs,
    keyPairs
  }
}

function makeKeyPair(json){
  return {
    name: json.name,
  };
}

function makeVpc(json){
  const [
    name,
    id,
    isDefault
  ] = at(json, [
    'tags.Name',
    'vpc_id',
    'is_default'
  ]);

  return {
    name: isDefault ? `${id} | ${name} (default)` : `${name} | ${id}`,
    id,
    isDefault,
  }
}

export default service;
