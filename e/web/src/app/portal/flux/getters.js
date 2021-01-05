import {requestStatus} from 'oss-app/flux/restApi/getters';
import { TRYING_TO_INIT_PORTAL } from './actionTypes';

export default {
  initPortalAttemp: requestStatus(TRYING_TO_INIT_PORTAL)
}
