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

export const SystemRoleEnum = {
  TELE_ADMIN: '@teleadmin'
}

export const ResourceEnum = {
  SAML: 'saml',
  OIDC: 'oidc',
  ROLE: 'role',
  AUTH_CONNECTORS: 'auth_connector',
  TRUSTED_CLUSTER: 'trusted_cluster',
  LOG_FWRD: 'logforwarder'
}

export const ErrorPageEnum = {
  FAILED_TO_LOGIN: 'login_failed',
  EXPIRED_LINK: 'expired_link',
  INVALID_USER: 'invalid_user',
  NOT_FOUND: 'not_found',
  FAILED: ''
}

export const AuthTypeEnum = {
  LOCAL: 'local',
  SSO: 'sso'
}

export const Auth2faTypeEnum = {
  UTF: 'u2f',
  OTP: 'otp',
  DISABLED: 'off'
}

export const AuthProviderTypeEnum = {
  OIDC: 'oidc',
  SAML: 'saml'
}

export const UserTokenTypeEnum = {
  RESET: 'reset',
  INVITE: 'invite'
}

export const ClusterNameEnum = {
  DevCluster: '___dev__cluster__'
}

export const RepositoryEnum = {
  SYSTEM: 'gravitational.io'
}

export const RemoteAccessEnum = {
  ON: 'on',
  OFF: 'off',
  NA: 'n/a'
}

export const RestRespCodeEnum = {
  FORBIDDEN: 403
}

export const ExpandPolicyEnum = {
  FIXED: 'fixed',
  FIXED_INSTANCE: 'fixed-instance'
}

export const UserStatusEnum = {
  INVITED: 'invited',
  ACTIVE: 'active'
}

export const SiteReasonEnum = {
  INVALID_LICENSE: 'license_invalid'
}

export const ServerVarEnums = {
  INTERFACE: 'interface',
  MOUNT: 'mount',
  DOCKER_DISK: 'docker_device',
  GRAVITY_DISK: 'system_device'
}

export const OpTypeEnum = {
  OPERATION_UPDATE: 'operation_update',
  OPERATION_INSTALL: 'operation_install',
  OPERATION_EXPAND: 'operation_expand',
  OPERATION_UNINSTALL: 'operation_uninstall',
  OPERATION_SHRINK: 'operation_shrink'
}

export const OpStateEnum = {
  FAILED: 'failed',
  CREATED: 'created',
  COMPLETED: 'completed',
  READY: 'ready',
  INSTALL_PRECHECKS: 'install_prechecks',
  INSTALL_INITIATED: 'install_initiated',
  INSTALL_SETTING_CLUSTER_PLAN: 'install_setting_plan',
  INSTALL_PROVISIONING: 'install_provisioning',
  INSTALL_DEPLOYING: 'install_deploying',
  UNINSTALL_IN_PROGRESS: 'uninstall_in_progress',
  EXPAND_PRECHECKS: 'expand_prechecks',
  EXPAND_INITIATED: 'expand_initiated',
  EXPAND_SETTING_PLAN: 'expand_setting_plan',
  EXPAND_PLANSET: 'expand_plan_set',
  EXPAND_PROVISIONING: 'expand_provisioning',
  EXPAND_DEPLOYING: 'expand_deploying',
  SHRINK_IN_PROGRESS: 'shrink_in_progress',
  UPDATE_IN_PROGRESS: 'update_in_progress'
}

export const SiteStateEnum = {
  ACTIVE: 'active',
  FAILED: 'failed',
  DEGRADED: 'degraded',
  NOT_INSTALLED: 'not_installed',
  INSTALLING: 'installing',
  UPDATING: 'updating',
  SHRINKING: 'shrinking',
  EXPANDING: 'expanding',
  UNINSTALLING: 'uninstalling',
  OFFLINE: 'offline'
}

export const ProviderEnum = {
  ONPREM: 'onprem',
  AZURE: 'azure',
  VAGRANT: 'vagrant',
  AWS: 'aws'
}

export const ProvisionerEnum = {
  ONPREM: 'onprem',
  AZURE: 'azure',
  VAGRANT: 'vagrant',
  AWS: 'aws_terraform'
}

export const K8sPodPhaseEnum = {
  SUCCEEDED: 'Succeeded',
  RUNNING: 'Running',
  PENDING: 'Pending',
  FAILED: 'Failed',
  UNKNOWN: 'Unknown'
}

export const K8sPodDisplayStatusEnum = {
  ...K8sPodPhaseEnum,
  TERMINATED: 'Terminated'
}

export const RetentionValueEnum = {
  DEF: 'default',
  MED: 'medium',
  LONG: 'long'
}

export const LinkEnum = {
  U2F_HELP_URL: 'https://support.google.com/accounts/answer/6103523?hl=en',
  DOC_OPSCENTER_PACKAGE: 'https://gravitational.com/telekube/docs/pack/',
  DOC_OPSCENTER_CLUSTER: 'https://gravitational.com/telekube/docs/manage/',
  DOC_CLUSTER_DELETE: 'https://gravitational.com/telekube/docs/cluster/#deleting-a-cluster',
  AWS_ACCESS_KEY: 'https://aws.amazon.com/developers/access-keys/',
  AWS_REGIONS: 'http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html',
  AWS_KEY_PAIRS: 'http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html',
  AWS_VPC: 'http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html',
  AWS_INSTANCE_TYPES: 'http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html',
  AWS_SESSION_TOKEN: 'http://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_request.html'
}
