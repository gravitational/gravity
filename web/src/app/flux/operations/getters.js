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

import { OpTypeEnum, OpStateEnum, ProvisionerEnum } from 'app/services/enums';

const ops = [ ['ops'], (ops) => ops ];

const isServerOp = type => type === OpTypeEnum.OPERATION_SHRINK || type === OpTypeEnum.OPERATION_EXPAND;

const opsBySiteId = (siteId) => [ ['op'], (ops) => {
  return ops.valueSeq()
    .filter(item=> item.get('site_id') === siteId)
    .sortBy(item=> item.get('created'))
    .reverse()
    .toArray();
}];

const serverOps = (siteId) => [ ['op'], (ops) => {
  return ops.valueSeq()
    .filter(item=> item.get('site_id') === siteId && isServerOp(item.get('type')))
    .sortBy(item=> item.get('created'))
    .reverse()
    .toArray();
}];

const updateOps = (siteId) => [ ['op'], (ops) => {
  return ops.valueSeq()
    .filter(item=> item.get('site_id') === siteId && item.get('type') === OpTypeEnum.OPERATION_UPDATE)
    .first()
    .toJS()
}];

const installOp = (siteId) => [ ['op'], (ops) => {
  return ops.valueSeq()
    .filter(item=> item.get('site_id') === siteId && item.get('type') === OpTypeEnum.OPERATION_INSTALL)
    .first()
    .toJS()
}];

const initiatedExpandOps = (siteId) => [ ['op'], (ops) => {
  return ops.valueSeq()
    .filter(item=>
      item.get('site_id') === siteId &&
      item.get('type') === OpTypeEnum.OPERATION_EXPAND &&
      item.get('state') === OpStateEnum.EXPAND_INITIATED)
    .sortBy(item=> item.get('created'))
    .reverse()
    .toArray();
}];

const lastShrinkOps = (siteId) => [ ['op'], (ops) => {
  return ops.valueSeq()
    .filter(item=> item.get('site_id') === siteId && item.get('type') === OpTypeEnum.OPERATION_SHRINK)
    .sortBy(item=> item.get('created'))
    .reverse()
    .toArray();
}];

const getOnPremInstructions = (opId) => ['op', opId, 'data', 'agents'];

const findInProgressUpdateOpsByAppId = (appId) => [ ['op'], (ops) => {
  let [repo, name, version] = appId.split('/');
  let pkgName = `${repo}/${name}:${version}`;
  return ops.valueSeq()
    .filter( item =>
        item.get('state') === OpStateEnum.UPDATE_IN_PROGRESS &&
        item.getIn(['data', 'update_package']) === pkgName &&
        item.get('type') === OpTypeEnum.OPERATION_UPDATE
      )
    .sortBy(item=> item.get('created'))
    .reverse()
    .toArray();
}];

const isOnPrem = opId => [['op', opId],
  opMap => opMap.get('provisioner') === ProvisionerEnum.ONPREM];

const opTypeById = opId => ['op', opId, 'type'];

export default {
  ops,
  opsBySiteId,
  opTypeById,
  installOp,
  initiatedExpandOps,
  lastShrinkOps,
  serverOps,
  updateOps,
  isOnPrem,
  getOnPremInstructions,
  findInProgressUpdateOpsByAppId
}