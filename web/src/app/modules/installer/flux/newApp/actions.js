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

import { at } from 'lodash';
import reactor from 'app/reactor';
import api from 'app/services/api';
import cfg from 'app/config';
import restApiActions from 'app/flux/restApi/actions';
import {checkIfNotEmptyString} from 'app/lib/paramUtils';
import getters from './getters';
import history from 'app/services/history';
import { ProvisionerEnum, ProviderEnum, RestRespCodeEnum  } from 'app/services/enums';
import { StepValueEnum } from './../enums';
import {CREATE_NEW_VPC_OPTION_VALUE} from './constants';
import {INSTALLER_SET_STEP} from './../installer/actionTypes';

import {
  INSTALLER_NEW_APP_INIT,
  INSTALLER_NEW_APP_SET_PROVIDER,
  INSTALLER_NEW_APP_SET_LICENSE,
  INSTALLER_NEW_APP_SET_DOMAIN_NAME,
  INSTALLER_NEW_APP_SET_DOMAIN_NAME_VALID_FLAG,
  INSTALLER_NEW_APP_SET_USE_EXISTING,
  INSTALLER_NEW_APP_SET_PROVIDER_KEYS,
  INSTALLER_NEW_APP_SET_PROVIDER_KEYS_VARIFIED,
  INSTALLER_NEW_APP_SET_KEY_PAIR_NAME,
  INSTALLER_NEW_APP_RECEIVE_PROVIDER_DATA,
  INSTALLER_NEW_APP_SET_AWS_REGION,
  INSTALLER_NEW_APP_SET_AWS_VPC,
  INSTALLER_NEW_APP_SET_TAGS,
  INSTALLER_NEW_APP_SET_ONPREM_CIDRS
 } from './actionTypes';

 import {
   TRYING_TO_VALIDATE_LICENSE,
   TRYING_TO_VALIDATE_DOMAIN_NAME,
   TRYING_TO_CREATE_NEW_SITE
  } from 'app/flux/restApi/constants';


function setLicense(license){
  reactor.dispatch(INSTALLER_NEW_APP_SET_LICENSE, license);
  reactor.dispatch(INSTALLER_SET_STEP, StepValueEnum.NEW_APP);
}

export function setAppTags(tags){
  reactor.dispatch(INSTALLER_NEW_APP_SET_TAGS, tags);
}

export function setOnpremSubnets(serviceSubnet, podSubnet){
  reactor.dispatch(INSTALLER_NEW_APP_SET_ONPREM_CIDRS, {
    serviceSubnet,
    podSubnet
  });
}

export function setDeploymentType(license, app_package){
  if(!license){
    setLicense(null);
    return;
  }

  restApiActions.start(TRYING_TO_VALIDATE_LICENSE);

  let data = {
    license,
    app_package
  }

  api.post(cfg.api.licenseValidationPath, data)
    .done(() => {
      restApiActions.success(TRYING_TO_VALIDATE_LICENSE);
      setLicense(license);
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      restApiActions.fail(TRYING_TO_VALIDATE_LICENSE, msg);
    })
}

export function createOnPremSite(request){
  let {
    serviceSubnet,
    podSubnet
    } = reactor.evaluate(getters.onpremProvider);

  request.provider = {
    provisioner: ProvisionerEnum.ONPREM,
    [ProviderEnum.ONPREM]: {
      pod_cidr: podSubnet,
      service_cidr: serviceSubnet
    }
  };

  _createSite(request);
}

export function createAwsSite(request){
  let {
    access_key,
    secret_key,
    session_token,
    keysVerified,
    useExisting,
    selectedVpc,
    selectedKeyPairName,
    selectedRegion} = reactor.evaluate(getters.awsProvider);

  let provisioner = useExisting ? ProvisionerEnum.ONPREM : null;
  let vpcValue = selectedVpc !== CREATE_NEW_VPC_OPTION_VALUE ? selectedVpc : null;

  if(!keysVerified){
    verifyKeys(access_key, secret_key, session_token);
  }else{
    request.provider = {
      provisioner,
      [ProviderEnum.AWS]: {
        access_key,
        key_pair: selectedKeyPairName,
        secret_key,
        session_token,
        region: selectedRegion,
        vpc_id: vpcValue
      }
    }

    _createSite(request);
  }
}

export function createSite(){
  let newApp = reactor.evaluate(getters.newApp);
  let { packageName, domainName, selectedProvider, license, tags } = newApp;
  let request = {
    app_package: packageName,
    domain_name: domainName,
    provider: null,
    license,
    labels: tags
  };

  try{
    if(!selectedProvider){
      throw Error('Please select provider');
    }

    if(selectedProvider === ProviderEnum.AWS){
      createAwsSite(request);
    }else if(selectedProvider === ProviderEnum.ONPREM){
      createOnPremSite(request);
    }else{
      throw Error('Unknown provider type');
    }
  } catch (err) {
    let msg = api.getErrorText(err);
    restApiActions.fail(TRYING_TO_CREATE_NEW_SITE, msg);
  }
}

export function useNewServers(useNew){
  reactor.dispatch(INSTALLER_NEW_APP_SET_USE_EXISTING, useNew);
}

export function setDomainNameVerifiedFlag(value){
  reactor.dispatch(INSTALLER_NEW_APP_SET_DOMAIN_NAME_VALID_FLAG, value);
}

export function setProvider(providerOption){
  reactor.dispatch(INSTALLER_NEW_APP_SET_PROVIDER, providerOption);
}

export function setProviderKeys(access_key, secret_key, session_token){
  reactor.dispatch(INSTALLER_NEW_APP_SET_PROVIDER_KEYS, {access_key, secret_key, session_token});
}

export function verifyKeys(access_key, secret_key, session_token){
  restApiActions.start(TRYING_TO_CREATE_NEW_SITE);
  let { packageName, selectedProvider } = reactor.evaluate(getters.newApp);
  let data = {
    provider: selectedProvider,
    variables: {
      access_key,
      secret_key,
      session_token
    },
    application: packageName
  };

  api.post(cfg.api.providerPath, data)
    .done((data)=>{
      reactor.batch(()=>{
        restApiActions.success(TRYING_TO_CREATE_NEW_SITE);
        reactor.dispatch(INSTALLER_NEW_APP_SET_PROVIDER_KEYS_VARIFIED, true);
        reactor.dispatch(INSTALLER_NEW_APP_RECEIVE_PROVIDER_DATA, data);
      })
    })
    .fail(err => {
      let msg = null;
      if (err.status === RestRespCodeEnum.FORBIDDEN) {
        msg = getIamPermissionErrorMsg(err);
      } else {
        msg = api.getErrorText(err);
      }

      restApiActions.fail(TRYING_TO_CREATE_NEW_SITE, msg);
    });
}

export function setDomainName(name){
  reactor.dispatch(INSTALLER_NEW_APP_SET_DOMAIN_NAME, name);
  restApiActions.start(TRYING_TO_VALIDATE_DOMAIN_NAME);
  api.get(cfg.getCheckDomainNameUrl(name))
    .done(data => {
      data = data || [];
      if(data.length > 0){
        restApiActions.fail(TRYING_TO_VALIDATE_DOMAIN_NAME, `${name} is already taken`);
      }else{
        restApiActions.success(TRYING_TO_VALIDATE_DOMAIN_NAME);
        reactor.dispatch(INSTALLER_NEW_APP_SET_DOMAIN_NAME_VALID_FLAG, true);
      }
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      restApiActions.fail(TRYING_TO_VALIDATE_DOMAIN_NAME, `Cannot validate domain name. ${msg}`);
    });
}

export function initNewApp({name, repository, version, licenseInfo }){
  checkIfNotEmptyString(name, "newApp.name");
  checkIfNotEmptyString(repository, "newApp.repository");
  checkIfNotEmptyString(version, "newApp.version");
  let packageName = `${repository}/${name}:${version}`;
  reactor.dispatch(INSTALLER_NEW_APP_INIT, {name, repository, version, packageName, licenseInfo });
}

export function setAwsRegion(regionName){
  reactor.dispatch(INSTALLER_NEW_APP_SET_AWS_REGION, regionName);
}

export function setAwsVpc(vpcName){
  reactor.dispatch(INSTALLER_NEW_APP_SET_AWS_VPC, vpcName);
}

export function setKeyPairName(key){
  reactor.dispatch(INSTALLER_NEW_APP_SET_KEY_PAIR_NAME, key);
}

const getIamPermissionErrorMsg = err => {
  let [actions] = at(err, 'responseJSON.error.actions');
  actions = actions || [];
  return {
    code: RestRespCodeEnum.FORBIDDEN,
    text: actions.join('\n')
  };
}

function _createSite(request){
  let url = cfg.getSiteUrl();
  restApiActions.start(TRYING_TO_CREATE_NEW_SITE);
  api.post(url, request)
    .done(json => {
      history.push(cfg.getInstallerProvisionUrl(json.site_domain), true);
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      restApiActions.fail(TRYING_TO_CREATE_NEW_SITE, msg);
    });
}
