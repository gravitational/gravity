import { getAcl } from 'oss-app/flux/userAcl';
import cfg from './../../config';

export function settingsAuth() {
  return getAcl().getConnectorAccess().list;
}

export function settingsRole() {
  return getAcl().getRoleAccess().list;
}

export function settingsLicense() {
  const allowed = getAcl().getLicenseAccess().create;
  return cfg.isSettingsLicenseGenEnabled() === true && allowed;
}