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

import reactor from 'app/reactor';
import cfg from 'app/config';
import { getAcl } from 'app/flux/userAcl';
import userGetters from 'app/flux/user/getters';

const hasK8sAccess = () => {
  return getAcl().getClusterAccess().connect;
}

export function siteMonitoring() {
  return hasK8sAccess() && cfg.isSiteMonitoringEnabled();
}

export function siteK8s() {
  return hasK8sAccess() && cfg.isSiteK8sEnabled();
}

export function siteConfigMaps() {
  return hasK8sAccess() && cfg.isSiteConfigMapsEnabled();
}

export function siteLogs() {
  return cfg.isSiteLogsEnabled();
}

export function settingsAccount(){
  const userStore = reactor.evaluate(userGetters.user)
  return !userStore.isSso();
}

export function siteRemoteAccess(){
  return cfg.isSiteRemoteAccessEnabled();
}

export function settingsCertificate() {
  return !cfg.isDevCluster();
}

export function settingsLogForwarder() {
  const allowed = getAcl().getLogForwarderAccess().list;
  return allowed && cfg.isSettingsLogsEnabled()
}

export function settingsMonitoring() {
  if (cfg.isDevCluster()) {
    return false;
  }

  return cfg.isSettingsMonitoringEnabled() && hasK8sAccess();
}

export function settingsUsers() {
  return getAcl().getUserAccess().list;
}
