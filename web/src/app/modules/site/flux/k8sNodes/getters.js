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

const nodeInfos = [['site_k8s_nodes'], nodeDataArray => {
  return nodeDataArray.map(itemMap => {    
    var condtion = findCondition(itemMap.getIn(['status', 'conditions']));
    return {
      nodeMap: itemMap,
      name: itemMap.getIn(['metadata', 'name']),
      status: itemMap.get('status').toJS(),
      condition: condtion,
      labels: itemMap.getIn(['metadata', 'labels']).toJS()
    }
  }).toJS();

}];
 
export default {  
  nodeInfos
}

function findCondition(conditions){
  var condition = {
    time: 'unknown',
    message: 'unknown status',
    icon: ''
  };

  conditions.forEach(item=>{
    if(item.get('type') === 'Ready'){
      if(item.get('status') == 'True'){
        condition = {
          message: 'ready',
          icon: 'text-success',
          time: item.get('lastHeartbeatTime')
        };

        return false;
      }

      condition = {
        icon: 'text-danger',
        message: item.get('message'),
        time: item.get('lastHeartbeatTime')
      };

      return false;
    }
  });

  return condition;
}