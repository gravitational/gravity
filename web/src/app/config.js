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
import _ from 'lodash';
import { isTestEnv } from './services/utils'
import { formatPattern, removeAllOptionalParameters } from 'app/lib/patternUtils';
import { ProviderEnum, ErrorPageEnum, ClusterNameEnum, Auth2faTypeEnum } from 'app/services/enums';
import Logger from 'app/lib/logger';

const logger = Logger.create('config/init');
const baseUrl = isTestEnv() ? 'localhost' : window.location.origin;

let cfg = {

  systemInfo: {
    wizard: false,
    serverVersion: {},
    clusterName: ClusterNameEnum.DevCluster
  },

  baseUrl,

  dateTimeFormat: 'DD/MM/YYYY HH:mm:ss',

  dateFormat: 'DD/MM/YYYY',

  auth: {
    second_factor: Auth2faTypeEnum.DISABLED,
    oids: [],
    saml: [],
  },

  user:{
    privacyPolicyUrl: 'http://gravitational.com/privacy',

    // logo to be displayed on login/forgot password screens
    logo: null,

    login: {
      headerText: 'Telekube'
    },

    completeRequest: {
      inviteHeaderText: 'Welcome',
      newPasswordHeaderText: 'User Reset'
    }
  },

  agentReport:{
    provision: {
      devices: {
        docker:{
          labelText: 'Docker device',
          tooltipText: 'Device provided to Docker for volume management'
        }
      },

      interfaces: {
        ipv4: {
          labelText: 'IP Address',
          toolipText: 'IP address used to communicate within the cluster'
        }
      },

      mounts: {
        /*
        *  'mount name': {
        *    labelText: 'input field label',
        *    toolipText: 'input field tooltip'
        * }
        */
      }
    }
  },

  routes: {
    // public routes
    app: '/web',
    login: '/web/login',
    logout: '/web/logout',
    userInvite: '/web/newuser/:token',
    userReset: '/web/reset/:token',
    errorPage: '/web/msg/error(/:type)',
    errorPageWithDetails: '/web/msg/error?details=:details',
    infoPage: '/web/msg/info(/:type)',

    // default app entry point
    defaultEntry: '/web/portal',

    // installer
    installerBase: '/web/installer',
    installerNewSite: '/web/installer/new/:repository/:name/:version',
    installerExistingSite: '/web/installer/site/:siteId',
    installerComplete: '/web/installer/site/:siteId/complete/',

    // settings
    settingsUsers: 'users',
    settingsAccount: 'account',
    settingsMetricsLogs: 'logs',
    settingsMetricsMonitor: 'monitoring',
    settingsTlsCert: 'cert',

    // site
    siteBase: '/web/site/:siteId',
    siteSettings: '/web/site/:siteId/settings',
    siteOffline: '/web/site/:siteId/offline',
    siteUninstall: '/web/site/:siteId/uninstall',
    siteApp: '/web/site/:siteId/app',
    siteConsole: '/web/site/:siteId/servers/console',
    siteServers: '/web/site/:siteId/servers',
    siteHistory: '/web/site/:siteId/history',
    siteLogs: '/web/site/:siteId/logs',
    siteMonitor: '/web/site/:siteId/monitor',
    siteMonitorPod: '/web/site/:siteId/monitor/dashboard/db/pods?var-namespace=:namespace&var-podname=:podName',
    siteConfiguration: '/web/site/:siteId/cfg',
    siteK8s: '/web/site/:siteId/k8s',
    siteK8sNodes: '/web/site/:siteId/k8s',
    siteK8sPods: '/web/site/:siteId/k8s/pods',
    siteK8sServices: '/web/site/:siteId/k8s/services',
    siteK8sJobs: '/web/site/:siteId/k8s/jobs',
    siteK8sDaemonSets: '/web/site/:siteId/k8s/daemons',
    siteK8sDeployments: '/web/site/:siteId/k8s/deployments',

    // sso redirects
    ssoOidcCallback: '/proxy/v1/webapi/*',
  },

  modules: {

    settings: {
      clusterHeaderText: 'Telekube Cluster',
      features: {
        logs: {
          enabled: true
          },

        monitoring: {
          enabled: true
        }
      }
    },

    site: {
      defaultNamespace: 'default',
      features: {
        remoteAccess: {
          enabled: false
        },
        logs: {
          enabled: true
        },
        k8s: {
          enabled: true
        },
        configMaps: {
          enabled: true
        },
        monitoring: {
          enabled: true,
          grafanaDefaultDashboardUrl: 'dashboard/db/cluster'
        }
      }
    },

    installer:{
      enableTags: true,
      eulaAgreeText: 'I Agree To The Terms',
      eulaHeaderText: 'Welcome to the {0} Installer',
      eulaContentLabelText: 'License Agreement',
      licenseHeaderText: 'Enter your license',
      licenseOptionTrialText: 'Trial without license',
      licenseOptionText: 'With a license',
      licenseUserHintText: `If you have a license, please insert it here. In the next steps you will select the location of your application and the capacity you need`,
      progressUserHintText: 'Your infrastructure is being provisioned and your application is being installed.\n\n Once the installation is complete you will be taken to your infrastructure where you can access your application.',
      prereqUserHintText: `If you select a cloud provider, we will automate the infrastructure provisioning on your account with your provided keys in the next step. Your keys are not stored on our system. \n\n If you select BareMetal we will provide you with a command to be run on each of your machines in the next step.`,
      provisionUserHintText: 'Drag the slider to estimate the number of resources needed for that performance level. You can also add / remove resources after the installation. \n\n Once you click "Start Installation" the resources will be provisioned on your infrastructure.',
      iamPermissionsHelpLink: 'https://gravitational.com/telekube/docs/overview/',

      providers: [ProviderEnum.AWS, ProviderEnum.ONPREM],
      providerSettings: {
        [ProviderEnum.AWS]: {
          useExisting: false
        }
       }
    }
  },

  api: {
    // portal
    appsPath: '/app/v1/applications/(:repository/:name/:version)',
    licenseValidationPath: '/portalapi/v1/license/validate',
    standaloneInstallerPath: '/portalapi/v1/apps/:repository/:name/:version/installer',
    appPkgFilePath:'/pack/v1/repositories/:repository/packages/:name/:version/file',
    appPkgUploadPath:'/portalapi/v1/apps',

    // provider
    providerPath: '/portalapi/v1/provider',

    // operations
    operationPath: '/portalapi/v1/sites/:siteId/operations(/:opId)',
    operationProgressPath: '/portalapi/v1/sites/:siteId/operations/:opId/progress',
    operationAgentPath: '/portalapi/v1/sites/:siteId/operations/:opId/agent',
    operationStartPath: '/portalapi/v1/sites/:siteId/operations/:opId/start',
    operationPrecheckPath: '/portalapi/v1/sites/:siteId/operations/:opId/prechecks',
    operationLogsPath: '/portal/v1/accounts/:accountId/sites/:siteId/operations/common/:opId/logs?access_token=:token',
    expandSitePath: '/portalapi/v1/sites/:siteId/expand',
    shrinkSitePath: '/portalapi/v1/sites/:siteId/shrink',

    // auth & session management
    ssoOidc: '/proxy/v1/webapi/oidc/login/web?redirect_url=:redirect&connector_id=:providerName',
    ssoSaml: '/proxy/v1/webapi/saml/sso?redirect_url=:redirect&connector_id=:providerName',
    renewTokenPath:'/proxy/v1/webapi/sessions/renew',
    sessionPath: '/proxy/v1/webapi/sessions',
    u2fCreateUserChallengePath: '/proxy/v1/webapi/u2f/signuptokens/:inviteToken',
    u2fCreateUserPath: '/proxy/v1/webapi/u2f/users',
    u2fSessionChallengePath: '/proxy/v1/webapi/u2f/signrequest',
    u2fSessionPath: '/proxy/v1/webapi/u2f/sessions',

    // user management
    checkDomainNamePath: '/portalapi/v1/domains/:domainName',
    inviteUserPath: '/portalapi/v1/sites/:siteId/invites',
    resetUserPath: '/portalapi/v1/sites/:siteId/users/:userId/reset',

    // user tokens
    userTokenInviteDonePath: '/portalapi/v1/tokens/invite/done',
    userTokenResetDonePath: '/portalapi/v1/tokens/reset/done',
    userTokenPath: '/portalapi/v1/tokens/user/:token',

    changeUserPswPath: '/portalapi/v1/accounts/existing/users/password',
    accountUsersPath: '/portalapi/v1/accounts/existing/users',
    accountDeleteUserPath: '/portalapi/v1/accounts/existing/users/:id',
    accountDeleteInvitePath: '/portalapi/v1/accounts/existing/invites/:id',
    userStatusPath: '/portalapi/v1/user/status',
    userContextPath: '/portalapi/v1/user/context',

    // terminal
    ttyWsAddr: ':fqdm/proxy/v1/webapi/sites/:cluster/connect?access_token=:token&params=:params',
    ttyWsK8sPodAddr: ':fqdm/portalapi/v1/sites/:cluster/connect?access_token=:token&params=:params',
    ttyEventWsAddr: ':fqdm/proxy/v1/webapi/sites/:cluster/sessions/:sid/events/stream?access_token=:token',
    ttyResizeUrl: '/proxy/v1/webapi/sites/:cluster/sessions/:sid',

    // site
    siteTlsCertPath: '/portalapi/v1/sites/:siteId/certificate',
    siteSessionPath: '/proxy/v1/webapi/sites/:siteId/sessions',
    siteNodesPath: '/proxy/v1/webapi/sites/:id/nodes',
    sitePath: '/portalapi/v1/sites(/:siteId)',
    siteReportPath: '/portalapi/v1/sites/:siteId/report',
    siteEndpointsPath: '/portalapi/v1/sites/:siteId/endpoints',
    siteServersPath: '/portalapi/v1/sites/:siteId/servers',
    siteLogAggegatorPath: '/sites/v1/:accountId/:siteId/proxy/master/logs/log?query=:query',
    siteDownloadLogPath: '/sites/v1/:accountId/:siteId/proxy/master/logs/download?query=:query',
    siteOperationReportPath: `/portal/v1/accounts/:accountId/sites/:siteId/operations/common/:opId/crash-report`,
    siteAppsPath: '/portalapi/v1/sites/:siteId/apps',
    siteFlavorsPath: '/portalapi/v1/sites/:siteId/flavors',
    siteUninstallStatusPath: '/portalapi/v1/sites/:siteId/uninstall',
    siteLicensePath: '/portalapi/v1/sites/:siteId/license',
    siteLogForwardersPath: '/portalapi/v1/sites/:siteId/logs/forwarders',
    siteMonitorRetentionValuesPath: '/portalapi/v1/sites/:siteId/monitoring/retention',
    siteRemoteAccessPath: '/portalapi/v1/sites/:siteId/access',
    siteGrafanaContextPath: '/portalapi/v1/sites/:siteId/grafana',
    siteResourcePath: '/portalapi/v1/sites/:siteId/resources(/:kind)',
    siteRemoveResourcePath: '/portalapi/v1/sites/:siteId/resources/:kind/:id',

    // kubernetes
    k8sNamespacePath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/api/v1/namespaces',
    k8sConfigMapsByNamespacePath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/api/v1/namespaces/:namespace/configmaps/:name',
    k8sConfigMapsPath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/api/v1/configmaps',
    k8sNodesPath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/api/v1/nodes',
    k8sPodsByNamespacePath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/api/v1/namespaces/:namespace/pods',
    k8sPodsPath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/api/v1/pods',
    k8sServicesPath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/api/v1/services',
    k8sJobsPath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/apis/batch/v1/jobs',
    k8sDelploymentsPath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/apis/extensions/v1beta1/deployments',
    k8sDaemonSetsPath: '/sites/v1/:accountId/:siteId/proxy/master/k8s/apis/extensions/v1beta1/daemonsets'
  },

  getWoopsyPageRoute(text){
    return formatPattern(cfg.routes.errorPageWithDetails, {
      type: ErrorPageEnum.FAILED,
      details: text
    })
  },

  getSiteSettingsRoute(siteId){
    return formatPattern(cfg.routes.siteSettings, {siteId});
  },

  getSiteK8sPodMonitorRoute(siteId, namespace, podName) {
    return formatPattern(cfg.routes.siteMonitorPod, {siteId, namespace, podName});
  },

  getSiteMonitorRoute(siteId){
    return formatPattern(cfg.routes.siteMonitor, {siteId});
  },

  getSiteK8sRoute(siteId){
    return formatPattern(cfg.routes.siteK8s, {siteId});
  },

  getSiteK8sNodesRoute(siteId){
    return formatPattern(cfg.routes.siteK8sNodes, {siteId});
  },

  getSiteK8sJobsRoute(siteId){
    return formatPattern(cfg.routes.siteK8sJobs, {siteId});
  },

  getSiteK8sPodsRoute(siteId){
    return formatPattern(cfg.routes.siteK8sPods, {siteId});
  },

  getSiteK8sServicesRoute(siteId){
    return formatPattern(cfg.routes.siteK8sServices, {siteId});
  },

  getSiteK8sDaemonsRoute(siteId){
    return formatPattern(cfg.routes.siteK8sDaemonSets, {siteId});
  },

  getSiteK8sDeploymentsRoute(siteId){
    return formatPattern(cfg.routes.siteK8sDeployments, {siteId});
  },

  getSiteConfigurationRoute(siteId){
    return formatPattern(cfg.routes.siteConfiguration, {siteId});
  },

  getSiteLogsRoute(siteId){
    return formatPattern(cfg.routes.siteLogs, {siteId});
  },

  getSiteUninstallRoute(siteId) {
    return formatPattern(cfg.routes.siteUninstall, { siteId });
  },

  getInstallNewSiteRoute(name, repository, version){
    return formatPattern(cfg.routes.installerNewSite,
       {name, repository, version});
  },

  getAppPkgFileUrl(name, repository, version){
    return formatPattern(cfg.api.appPkgFilePath, {name, repository, version});
  },

  getStandaloneInstallerPath(name, repository, version){
    return formatPattern(cfg.api.standaloneInstallerPath, {name, repository, version});
  },

  getSiteMonitorRetentionValuesUrl(siteId){
    return formatPattern(cfg.api.siteMonitorRetentionValuesPath, {siteId});
  },

  getSiteServersUrl(siteId){
    return formatPattern(cfg.api.siteServersPath, {siteId});
  },

  getSiteResourcesUrl(siteId, kind){
    let path = cfg.api.siteResourcePath;
    if (!kind) {
      path = removeAllOptionalParameters(path);
    }
    return formatPattern(path, { siteId, kind });
  },

  getSiteRemoveResourcesUrl(siteId, kind, id){
    return formatPattern(cfg.api.siteRemoveResourcePath, { siteId, kind, id })
  },

  getSiteReportUrl(siteId){
    return formatPattern(cfg.api.siteReportPath, {siteId});
  },

  getSiteOperationReportUrl(accountId, siteId, opId){
    return formatPattern(cfg.api.siteOperationReportPath, {
      accountId,
      siteId,
      opId
    });
  },

  getSiteRemoteAccessUrl(siteId){
    return formatPattern(cfg.api.siteRemoteAccessPath, {siteId});
  },

  getSiteEndpointsUrl(siteId){
    return formatPattern(cfg.api.siteEndpointsPath, {siteId});
  },

  getSiteLogForwardersUrl(siteId){
    return formatPattern(cfg.api.siteLogForwardersPath, {siteId});
  },

  getSiteDownloadLogUrl(siteId, accountId, query) {
    return formatPattern(cfg.api.siteDownloadLogPath, {
      siteId,
      accountId,
      query
    });
  },

  getSiteLogAggregatorUrl(siteId, accountId, query) {
    return formatPattern(cfg.api.siteLogAggegatorPath, {
      siteId,
      accountId,
      query
    });
  },

  getExpandSiteUrl(siteId){
    return formatPattern(cfg.api.expandSitePath, {siteId});
  },

  getShrinkSiteUrl(siteId){
    return formatPattern(cfg.api.shrinkSitePath, {siteId});
  },

  getAppsUrl(name, repository, version){
    let url = formatPattern(cfg.api.appsPath, {name, repository, version});
    return url.replace(/\/$/, '');
  },

  getOperationUrl(siteId, opId){
    return formatPattern(cfg.api.operationPath, {siteId, opId}).replace(/\/$/g, '');
  },

  getOperationAgentUrl(siteId, opId){
    return formatPattern(cfg.api.operationAgentPath, {siteId, opId});
  },

  getOperationProgressUrl(siteId, opId){
    return formatPattern(cfg.api.operationProgressPath, {siteId, opId});
  },

  getOperationStartUrl(siteId, opId){
    return formatPattern(cfg.api.operationStartPath, {siteId, opId});
  },

  operationPrecheckPath(siteId, opId){
    return formatPattern(cfg.api.operationPrecheckPath, {siteId, opId});
  },

  getSiteUrl(siteId){
    return formatPattern(cfg.api.sitePath, {siteId}).replace(/\/$/g, '');
  },

  getSiteGrafanaContextUrl(siteId){
    return formatPattern(cfg.api.siteGrafanaContextPath, {siteId});
  },

  getSiteLicenseUrl(siteId){
    return formatPattern(cfg.api.siteLicensePath, {siteId});
  },

  getSiteFlavorsUrl(siteId){
    return formatPattern(cfg.api.siteFlavorsPath, {siteId});
  },

  getSiteAppsUrl(siteId){
    return formatPattern(cfg.api.siteAppsPath, {siteId});
  },

  getSiteUninstallStatusUrl(siteId){
    return formatPattern(cfg.api.siteUninstallStatusPath, {siteId});
  },

  getInstallerProvisionUrl(siteId){
    return formatPattern(cfg.routes.installerExistingSite, {siteId});
  },

  getInstallerLastStepUrl(siteId){
    return formatPattern(cfg.routes.installerComplete, {siteId});
  },

  getCheckDomainNameUrl(domainName){
    return formatPattern(cfg.api.checkDomainNamePath, {domainName})
  },

  getNodesUrl(id){
    return formatPattern(cfg.api.siteNodesPath, {id});
  },

  getAccountDeleteInviteUrl(id){
    return formatPattern(cfg.api.accountDeleteInvitePath, {id});
  },

  getAccountDeleteUserUrl(id){
    return formatPattern(cfg.api.accountDeleteUserPath, {id});
  },

  getUserRequestInfo(token){
    return formatPattern(cfg.api.userTokenPath, {token});
  },

  getSsoUrl(providerUrl, providerName, redirect) {
    return cfg.baseUrl + "/proxy" + formatPattern(providerUrl, { redirect, providerName });
  },

  getAuth2faType() {
    let [secondFactor=null] = _.at(cfg, 'auth.second_factor');
    return secondFactor;
  },

  getU2fCreateUserChallengeUrl(inviteToken){
    return formatPattern(cfg.api.u2fCreateUserChallengePath, {inviteToken});
  },


  getAuthProviders() {
    return cfg.auth && cfg.auth.providers ? cfg.auth.providers : [];
  },

  is2FAEnabled(){
    return cfg.auth.twoFA === true;
  },

  getSiteRoute(siteId){
    return formatPattern(cfg.routes.siteBase, {siteId});
  },

  getSiteAppUrl(siteId){
    return formatPattern(cfg.routes.siteApp, {siteId});
  },

  getSiteServersRoute(siteId){
    return formatPattern(cfg.routes.siteServers, {siteId});
  },

  getSiteConsoleRoute(siteId){
    return formatPattern(cfg.routes.siteConsole, {siteId});
  },

  getSiteHistoryRoute(siteId){
    return formatPattern(cfg.routes.siteHistory, {siteId});
  },

  getSiteLogQueryRoute(siteId, query){
    let route = cfg.routes.siteLogs;
    return formatPattern(`${route}?query=:query`, {siteId, query});
  },

  getSiteTlsCertUrl(siteId) {
    return formatPattern(cfg.api.siteTlsCertPath, { siteId });
  },

  getSiteUserInvitePath(siteId) {
    return formatPattern(cfg.api.inviteUserPath, { siteId });
  },

  getSiteUserResetPath(siteId, userId) {
    return formatPattern(cfg.api.resetUserPath, { siteId, userId });
  },

  getSiteSessionUrl(siteId){
    return formatPattern(cfg.api.siteSessionPath, {siteId});
  },

  getSiteInstallerRoute(siteId){
    return formatPattern(cfg.routes.installerExistingSite, {siteId});
  },

  getSiteDefaultDashboard() {
    let [suffix] = _.at(cfg, 'modules.site.features.monitoring.grafanaDefaultDashboardUrl');
    return suffix;
  },

  getWsHostName(){
    const prefix = location.protocol === 'https:' ? 'wss://' : 'ws://';
    const hostport = location.hostname+(location.port ? ':'+location.port: '');
    return `${prefix}${hostport}`;
  },

  getServerVersion() {
    let [serverVer] = _.at(cfg, 'systemInfo.serverVersion');
    return {
      version: serverVer.version,
      gitCommit: serverVer.gitCommit,
      gitTreeState: serverVer.gitTreeState
    }
  },

  isSettingsLogsEnabled() {
    let [isLogsEnabled] = _.at(cfg, 'modules.settings.features.logs.enabled');
    return isLogsEnabled;
  },

  isSettingsMonitoringEnabled() {
    let [isMonitoringEnabled] = _.at(cfg, 'modules.settings.features.monitoring.enabled');
    return isMonitoringEnabled;
  },

  getAgentDeviceDocker() {
    let [ option ] = _.at(cfg, 'agentReport.provision.devices.docker');
    return option || {};
  },

  getAgentDeviceMount(name) {
    let [ option ] = _.at(cfg, `agentReport.provision.mounts.${name}`);
    return option || {};
  },

  getAgentDeviceIpv4() {
    let [ option ] = _.at(cfg, 'agentReport.provision.interfaces.ipv4');
    return option || {};
  },

  enableSettingsMonitoring(value=true){
    cfg.modules.settings.features.monitoring.enabled = value;
  },

  enableSettingsLogs(value=true){
    cfg.modules.settings.features.logs.enabled = value;
  },

  enableSiteMonitoring(value=true){
    cfg.modules.site.features.monitoring.enabled = value;
  },

  enableSiteK8s(value=true){
    cfg.modules.site.features.k8s.enabled = value;
  },

  enableSiteConfigMaps(value = true) {
    cfg.modules.site.features.configMaps.enabled = value;
  },

  enableSiteLogs(value = true) {
    cfg.modules.site.features.logs.enabled = value;
  },

  isSiteRemoteAccessEnabled(){
    return cfg.modules.site.features.remoteAccess.enabled;
  },

  isSiteMonitoringEnabled(){
    return cfg.modules.site.features.monitoring.enabled;
  },

  isSiteK8sEnabled(){
    return cfg.modules.site.features.k8s.enabled;
  },

  isSiteConfigMapsEnabled(){
    return cfg.modules.site.features.configMaps.enabled;
  },

  isSiteLogsEnabled(){
    return cfg.modules.site.features.logs.enabled;
  },

  isStandAlone() {
    let [wizard] = _.at(cfg, 'systemInfo.wizard');
    return wizard;
  },

  /**
   * getLocalSiteId returns local cluster id.
   * for OpsCenter, accessing a remote cluster, it will be OpsCenter siteId.
   * for OpsCenter, running in dev mode i.e. without k8s, it will be ClusterNameEnum.DevCluster.
   */
  getLocalSiteId() {
    let [siteId] = _.at(cfg, 'systemInfo.clusterName');
    return siteId;
  },

  getSettingsClusterHeaderText(){
    let [headerText] = _.at(cfg, 'modules.settings.clusterHeaderText');
    return headerText;
  },

  isRemoteAccess(siteId) {
    return cfg.getLocalSiteId() !== siteId;
  },

  isDevCluster(){
    return cfg.systemInfo.clusterName === ClusterNameEnum.DevCluster;
  },

  setServerVersion(ver={}){
    cfg.systemInfo.serverVersion = ver;
    logger.info("server version", ver);
  },

  init(config={}){
    $.extend(true, this, config);
  }
}

export default cfg;
