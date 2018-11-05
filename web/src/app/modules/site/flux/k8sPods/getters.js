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

import { uniqBy } from 'lodash';
import reactor from 'app/reactor';
import cfg from 'app/config';
import { K8sPodPhaseEnum, K8sPodDisplayStatusEnum } from 'app/services/enums'
import currentSiteGetters from './../currentSite/getters';

const DEFAULT_NAMESPACE = cfg.modules.site.defaultNamespace;

const podInfoList = [
  ['site_k8s_pods'],
  (podsMap) => {
    return podsMap.valueSeq()
      .filter(itemMap => itemMap.getIn(['status', 'phase']) !== K8sPodPhaseEnum.SUCCEEDED)
      .map(itemMap => {
        const siteId = reactor.evaluate(currentSiteGetters.getSiteId);
        const podName = itemMap.getIn(['metadata','name']);
        const podNamespace = itemMap.getIn(['metadata', 'namespace']);
        const podLogUrl = cfg.getSiteLogQueryRoute(siteId, `pod:${podName}`);
        const podMonitorUrl = cfg.getSiteK8sPodMonitorRoute(siteId, podNamespace, podName);
        const { status, statusDisplay } = getStatus(itemMap);
        return {
          podMap: itemMap,
          podLogUrl,
          podMonitorUrl,
          podName,
          podNamespace,
          podHostIp: itemMap.getIn(['status','hostIP']),
          podIp: itemMap.getIn(['status','podIP']),
          containers: getContainers(itemMap),
          containerNames: getContainerNames(itemMap),
          labelsText: getLabelsText(itemMap),
          status,
          statusDisplay,
          phaseValue: itemMap.getIn(['status', 'phase'])
        }
      })
      .toJS();
  }
];

const autoCompleteOptions = [ ['site_k8s_pods'], (podsMap) => {
  let containerNames = [];

  let podOptions = podsMap
    .valueSeq()
    .filter( itemMap=> itemMap.getIn(['metadata', 'namespace']) === DEFAULT_NAMESPACE )
    .map( itemMap => {
      // store pod name
      containerNames.push(...getContainerNames(itemMap))
      return {
        type: 'pod',
        text: itemMap.getIn(['metadata','name']),
        details: {
          podHostIp: itemMap.getIn(['status','hostIP']),
          podIp: itemMap.getIn(['status','podIP'])
        }
      }
    })
    .toJS();

  let containerOptions = containerNames.map( text => ({
    type: 'container',
    text
  }));

  let allOptions = [...podOptions, ...containerOptions];

  return uniqBy(allOptions, 'text');

} ];

export default {
  podInfoList,
  autoCompleteOptions
}

// helpers
function createContainerStatus(containerMap){
  let phaseText = 'unknown';
  if(containerMap.getIn(['state', 'running'])){
    phaseText = 'running';
  }

  let name = containerMap.get('name');
  let siteId = reactor.evaluate(currentSiteGetters.getSiteId);
  let logUrl = cfg.getSiteLogQueryRoute(siteId, `container:${name}`);

  return {
    name,
    logUrl,
    phaseText
  }
}

function getLabelsText(pod){
  let labelMap = pod.getIn(['metadata', 'labels']);
  if(!labelMap){
    return [];
  }

  let results = [];
  let withAppAndName = [];

  labelMap.entrySeq().forEach(item => {
    let [labelName, lavelValue] = item;
    let text = labelName+':'+ lavelValue;
    if(labelName === 'app' || labelName === 'name' ){
      withAppAndName.push(text);
    }else{
      results.push(text);
    }
  })

 return withAppAndName.concat(results);

}

function getContainers(pod){
  const statusList = pod.getIn(['status', 'containerStatuses']);
  if(!statusList){
    return [];
  }

  return statusList
    .map(createContainerStatus)
    .toArray() || [];
}

function getContainerNames(podMap){
  const containerList = podMap.getIn(['spec', 'containers']);
  if(!containerList){
    return [];
  }

  return containerList
    .map(item=> item.get('name'))
    .toArray() || [];
}


function getStatus(pod) {
  // See k8s dashboard js logic
  // https://github.com/kubernetes/dashboard/blob/f63003113555ecf489b2a737797913a045b218c3/src/app/frontend/pod/list/card_component.js#L109
  let podStatus = pod.getIn(['status', 'phase']);
  let statusDisplay = podStatus;
  let reason = undefined;
  const statuses = pod.getIn(['status', 'containerStatuses']);
  if (statuses) {
    statuses.reverse().forEach(status => {
      const waiting = status.get('waiting');
      if (waiting) {
        podStatus = K8sPodDisplayStatusEnum.PENDING;
        reason = waiting.get('reason');
      }

      const terminated = status.get('terminated');
      if (terminated) {
        const terminatedSignal = terminated.get('signal');
        const terminatedExitCode = terminated.get('exitCode');
        const terminatedReason = terminated.get('reason');
        podStatus = K8sPodDisplayStatusEnum.TERMINATED;
        reason = terminatedReason;
        if (!reason) {
          if (terminatedSignal) {
            reason = `signal:${terminatedSignal}`;
          } else {
            reason = `exitCode:${terminatedExitCode}`;
          }
        }
      }
    });
  }

  if (podStatus === K8sPodDisplayStatusEnum.PENDING) {
    statusDisplay = `Waiting: ${reason}`;
  }
  if (podStatus === K8sPodDisplayStatusEnum.TERMINATED) {
    statusDisplay = `Terminated: ${reason}`;
  }
  return {
    status: podStatus,
    statusDisplay: statusDisplay
  }
}