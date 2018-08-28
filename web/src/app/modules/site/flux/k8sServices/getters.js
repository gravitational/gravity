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

const serviceInfoList = [
  ['site_k8s_services'],
  (servicesMap) => {
    return servicesMap.valueSeq().map(itemMap=>{
      return {
        serviceMap: itemMap,
        serviceNamespace: itemMap.getIn(['metadata', 'namespace']),
        serviceName: itemMap.getIn(['metadata','name']),
        clusterIp: itemMap.getIn(['spec','clusterIP']),
        labels: getLabelsText(itemMap),
        ports: getPorts(itemMap.getIn(['spec', 'ports']))
      };
    }).toJS();
  }
];

export default {
  serviceInfoList
}

function getPorts(ports = []){
  return ports
    .map(item=> `${item.get('protocol')}:${item.get('port')}/${item.get('targetPort')}`)
    .toArray()
}

function getLabelsText(service){
  var labels = service.getIn(['metadata', 'labels']);
  if(!labels){
    return [];
  }

  return labels
     .entrySeq()
     .map(item => item[0]+':'+item[1])
     .toArray();
}
