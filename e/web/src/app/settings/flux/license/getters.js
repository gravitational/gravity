import { requestStatus } from 'oss-app/flux/restApi/getters';
import { TRYING_TO_CREATE_LICENSE } from './actionTypes';

export default {
  createLicenseAttempt: requestStatus(TRYING_TO_CREATE_LICENSE)
}