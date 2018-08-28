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

import namespaceGetters from './../k8sNamespaces/getters';
import { displayK8sAge } from 'app/lib/dateUtils';

export default {
  k8s: [['site_k8s'], map => map.toJS() ],
	k8sJobs: [['site_k8s_jobs'], getJobs ],
  k8sDaemonSets: [['site_k8s_daemonsets'], getDaemonSets ],
  k8sDeployments: [['site_k8s_deployments'], getDeployments ],
  namespaces: namespaceGetters.namespaceNames  
}

function getDeployments(deploymentList) {  
	return deploymentList.map(itemMap => {              
      const created = itemMap.getIn(['metadata', 'creationTimestamp']);
      return {
        nodeMap: itemMap,
        name: itemMap.getIn(['metadata', 'name']),
        namespace: itemMap.getIn(['metadata', 'namespace']),        
        created: new Date(created),
        createdDisplay: displayK8sAge(created),
        desired: itemMap.getIn(['spec', 'replicas']),
        statusCurrentReplicas: itemMap.getIn(['status', 'replicas']),
        statusUpdatedReplicas: itemMap.getIn(['status', 'updatedReplicas']),
        statusAvailableReplicas: itemMap.getIn(['status', 'availableReplicas'])        
      }
    }).toJS();    
	}

function getDaemonSets(daemonList) {  
	return daemonList.map(itemMap => {        
      const created = itemMap.getIn(['metadata', 'creationTimestamp']);
      return {
        nodeMap: itemMap,
        name: itemMap.getIn(['metadata', 'name']),
        namespace: itemMap.getIn(['metadata', 'namespace']),        
        created: new Date(created),        
        createdDisplay: displayK8sAge(created),        
        statusCurrentNumberScheduled: itemMap.getIn(['status', 'currentNumberScheduled']),
				statusNumberMisscheduled: itemMap.getIn(['status', 'numberMisscheduled']),
				statusNumberReady: itemMap.getIn(['status', 'numberReady']),
        statusDesiredNumberScheduled: itemMap.getIn(['status', 'desiredNumberScheduled'])            
      }
    }).toJS();    
	}
	
function getJobs(jobList) {  
  return jobList.map(itemMap => {                    
      const created = itemMap.getIn(['metadata', 'creationTimestamp']);
      return {
        nodeMap: itemMap,
        name: itemMap.getIn(['metadata', 'name']),
        namespace: itemMap.getIn(['metadata', 'namespace']),        
        created: new Date(created),
        createdDisplay: displayK8sAge(created),
        desired: itemMap.getIn(['spec', 'completions']),
        statusSucceeded: itemMap.getIn(['status', 'succeeded']),
        statusFailed: itemMap.getIn(['status', 'failed']),
        statusActive: itemMap.getIn(['status', 'active'])            
      }
    }).toJS();    
  }