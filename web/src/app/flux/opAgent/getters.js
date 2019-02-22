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

import { toImmutable } from 'nuclear-js';
import { ServerVarEnums } from 'app/services/enums';
import { DISK_OPTION_AUTOMATIC } from './constants';
import humanize from 'humanize';

const NEEDS_DOCKER_DISK = 'devicemapper';

const serverCountByOp = opId => [['opAgent', opId, 'servers'], serverList => {
  if (serverList) {
    return serverList.size;
  }

  return 0;
}]

const serverConfigs = [['opAgentServers', 'servers'], srvMap => {
  let allServerConfigs = [];
  srvMap.valueSeq().forEach( serversByRoleList => {
    let serverConfigs = serversByRoleList
     .map(itemMap=>{
       let hostname = itemMap.get('hostName');
       let role = itemMap.get('serverRole');
       let varsList = itemMap.get('vars') || toImmutable([]);
       let advertise_ip = null;
       let mounts = [];
       let docker = null;
       let system_state = null;
       let os = itemMap.get('osInfo').toJS();

       varsList.forEach(varMap=>{
         let type = varMap.get('type');

         switch (type) {
           case ServerVarEnums.DOCKER_DISK:
             docker = getDiskVar(varMap);
             return;

           case ServerVarEnums.INTERFACE:
             advertise_ip = varMap.get('value');
             return;

           case ServerVarEnums.MOUNT:
             mounts.push({
               name: varMap.get('name'),
               source: varMap.get('value')
             });
             return;

           default:
             return;
         }
       })

       return {
         os,
         role,
         system_state,
         advertise_ip,
         hostname,
         docker,
         mounts
       }
     })
     .toJS()

     allServerConfigs = allServerConfigs.concat(serverConfigs)
  })

  return allServerConfigs;
}]

const serverVarsByRole = (opId, role) => [ ['opAgent', opId], reportMap => {
  if(!reportMap || !reportMap.get('servers') ){
    return [];
  }

  let serverList = reportMap.get('servers')
  let hasDocker = reportMap.getIn(['docker', 'storageDriver']) === NEEDS_DOCKER_DISK;

  return serverList
    .filter(itemMap => itemMap.get('role') === role)
    .map(itemMap => {
      let diskVars = [];
      let mountVars = createMountVars(itemMap);
      let interfaceVars = createInterfaceVars(itemMap);

      if(hasDocker){
        diskVars = createDockerVars(itemMap);
      }

      let vars = [...interfaceVars, ...diskVars, ...mountVars];
      let osInfo = itemMap.get('os').toJS();

      return {
        serverRole: itemMap.get('role'),
        hostName: itemMap.get('hostname'),
        vars,
        osInfo
      }
    })
    .sortBy(item => item.hostName)
    .toJS();
}];

function getDiskVar(varMap){
  let disk = varMap.get('value');
  let device = {}
  if(disk !== DISK_OPTION_AUTOMATIC){
    device = {name: disk}
  }

  return {
    device
  }
}

function createDockerVars(serverMap){
  let vars = [];
  let diskList = serverMap.get('devices') || toImmutable([]);

  // options for the variables
  let options = diskList.map(diskMap=>{
    let disk = diskMap.toJS();
    let {type, name, size_mbytes=0} = disk;
    let size = humanize.filesize(size_mbytes * 1024 * 1024);

    return {
      value: name,
      label: `${name}|${type}|${size}`
    }
  }).toJS();

  // add an option for 'automatic'
  options.unshift({
    value: DISK_OPTION_AUTOMATIC,
    label: DISK_OPTION_AUTOMATIC
  })

  vars.push({
    options,
    // default value is automatic
    value: DISK_OPTION_AUTOMATIC,
    type: ServerVarEnums.DOCKER_DISK
  })

  return vars;
}

function createInterfaceVars(serverMap){
  let options = serverMap.get('interfaces')
    .valueSeq()
    .map(intrMap => {
      return intrMap.get('ipv4_addr');
    })
    .toJS();

  let defaultValue = serverMap.getIn(['advertise_addr']);
  if(!defaultValue && options.length === 1){
    defaultValue = options[0];
  }

  return [{
    type: ServerVarEnums.INTERFACE,
    value: defaultValue,
    options
  }]
}

function createMountVars(serverMap){
  let mounts = serverMap.get('mounts') || toImmutable([]);

  return mounts.map(itemMap=>{
      let options = [];
      let name = itemMap.get('name');
      return {
        name,
        type: ServerVarEnums.MOUNT,
        value: itemMap.get('source'),
        options
      }
    })
    .toJS();
}

export default {
  serverCountByOp,
  serverVarsByRole,
  serverConfigs
}
