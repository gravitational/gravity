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
import currentSiteStore from './currentSite/currentSiteStore';
import configMapStore from './k8sConfigMaps/configMapStore';
import k8sStore from './k8s/k8sStore';
import k8sJobsStore from './k8s/jobsStore';
import k8sDaemonsetsStore from './k8s/daemonSetsStore';
import k8sDeploymentStore from './k8s/deploymentsStore';
import k8sNodeStore from './k8sNodes/nodeStore';
import k8sNamespaceStore from './k8sNamespaces/namespaceStore';
import k8sPodStore from './k8sPods/podStore';
import k8sServiceStore from './k8sServices/serviceStore';
import uninstallerStatusStore from './uninstall/statusStore';
import masterConsoleStore from './masterConsole/masterConsoleStore';
import serverStore from './servers/serverStore';
import serverProvisionerStore from './servers/serverProvisionerStore';
import serverDialogsStore from './servers/serverDialogStore';
import historyStore from './history/historyStore';

reactor.registerStores({
  'site_current': currentSiteStore,
  'site_config_maps': configMapStore,  
  'site_k8s': k8sStore,
  'site_k8s_jobs': k8sJobsStore,
  'site_k8s_daemonsets': k8sDaemonsetsStore,
  'site_k8s_deployments': k8sDeploymentStore,
  'site_k8s_nodes': k8sNodeStore,
  'site_k8s_namespaces': k8sNamespaceStore,
  'site_k8s_pods': k8sPodStore,
  'site_k8s_services': k8sServiceStore,
  'site_uninstaller_status': uninstallerStatusStore,
  'site_master_console': masterConsoleStore,
  'site_servers': serverStore,
  'site_servers_provisioner': serverProvisionerStore,
  'site_servers_dialogs': serverDialogsStore,
  'site_history': historyStore
});
