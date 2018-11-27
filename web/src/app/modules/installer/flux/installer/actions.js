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

import $ from 'jQuery';
import { at } from 'lodash';
import cfg from 'app/config';
import reactor from 'app/reactor';
import api from 'app/services/api';
import { parseWebConfig } from 'app/lib/paramUtils';
import restApiActions from 'app/flux/restApi/actions';
import Logger from 'app/lib/logger';
import { fetchSites, fetchFlavors, applyConfig } from 'app/flux/sites/actions';
import { fetchOps } from 'app/flux/operations/actions';
import { fetchApps } from 'app/flux/apps/actions';
import opGetters from 'app/flux/operations/getters';
import siteGetters from 'app/flux/sites/getters';
import { OpStateEnum,  }  from 'app/services/enums';
import { initNewApp } from './../newApp/actions';
import { initProvision } from './../provision/actions';
import { StepValueEnum } from './../enums';
import { INSTALLER_INIT, INSTALLER_EULA_ACCEPT }  from './actionTypes';
import { TRYING_TO_INIT_INSTALLER } from 'app/flux/restApi/constants';

const logger = Logger.create('modules/installer/actions');

export function initWithApp(nextState){
  let {name, repository, version } = nextState.params;
  tryToInit(
    fetchApps(name, repository, version)
      .then( app => {
        try{
          let customConfig = getCustomInstallerCfg(app);
          let licenseInfo =  getLicense(app);
          let eula = getEula(app);
          let requiresLicense = licenseInfo.enabled;
          let step = requiresLicense ? StepValueEnum.LICENSE : StepValueEnum.NEW_APP;
          let displayName = getAppDisplayName(app);

          displayName = displayName || name;

          reactor.dispatch(INSTALLER_INIT, {
            step,
            name,
            displayName,
            repository,
            version,
            customConfig,
            requiresLicense,
            eula
          });

          initNewApp({
            name,
            repository,
            version,
            licenseInfo
          });

          restApiActions.success(TRYING_TO_INIT_INSTALLER);

        }catch(err){
          return $.Deferred().reject(err);
        }
      })
  );
}

export function acceptEula(){
  reactor.dispatch(INSTALLER_EULA_ACCEPT);
}

export function initWithSite(nextState){
  let {siteId} = nextState.params;

  tryToInit(
    $.when(
      fetchFlavors(siteId),
      fetchSites(siteId),
      fetchOps(siteId))
    .then((res)=>{
      try{
        let [flavors] = res;
        let installOp = reactor.evaluate(opGetters.installOp(siteId));
        let site = reactor.evaluate(siteGetters.siteById(siteId));
        let opId = installOp.id;
        let opState = installOp.state;
        let requiresLicense = getLicense(site).enabled;
        let step = mapOpStateToStep(opState);

        applyConfig(siteId);

        reactor.dispatch(INSTALLER_INIT, {
          step,
          siteId,
          opId,
          requiresLicense
        });

        if( step === StepValueEnum.PROVISION ){
          initProvision(siteId, opId, flavors);
        }

        restApiActions.success(TRYING_TO_INIT_INSTALLER);

      }catch(err) {
        return $.Deferred().reject(err);
      }
    })
  );
}

// TODO: too ugly
function getCustomInstallerCfg(app) {
  let [webConfigJsonStr] = at(app, 'manifest.webConfig');
  let [providers = {}] = at(app, 'manifest.providers');

  let customWebCfg = parseWebConfig(webConfigJsonStr);
  let [customInstallModuleCfg] = at(customWebCfg, 'modules.installer');
  let [defaultProviders] = at(cfg, 'modules.installer.providers');

  defaultProviders = defaultProviders || [];
  customInstallModuleCfg = customInstallModuleCfg || {};

  customInstallModuleCfg.providers = defaultProviders.filter(k =>
    !providers[k] || providers[k].disabled !== true);

  return customInstallModuleCfg;
}

function getLicense(obj){
  let [license={}] = at(obj, 'manifest.license');
  return license;
}

function getEula(obj) {
  let [content = null] = at(obj, 'manifest.installer.eula.source');
  let enabled = !!content;
  return { enabled, content };
}

function tryToInit(promise){
  restApiActions.start(TRYING_TO_INIT_INSTALLER);
  promise
    .fail(err => {
      let msg = api.getErrorText(err);
      logger.error('installer failed to initialize', err);
      restApiActions.fail(TRYING_TO_INIT_INSTALLER, msg);
    })
}

function getAppDisplayName(obj){
  let [displayName] = at(obj, 'manifest.metadata.displayName');
  return displayName;
}

function mapOpStateToStep(opState){
  let step;
  switch (opState) {
    case OpStateEnum.CREATED:
    case OpStateEnum.INSTALL_INITIATED:
    case OpStateEnum.INSTALL_PRECHECKS:
    case OpStateEnum.INSTALL_SETTING_CLUSTER_PLAN:
      step = StepValueEnum.PROVISION;
      break;
    default:
      step = StepValueEnum.PROGRESS;
  }

  return step;
}