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
import { isObject } from 'lodash';
import reactor from 'app/reactor';
import { RestRespCodeEnum } from 'app/services/enums';
import withFeature from './../components/withFeature';
import api from 'app/services/api';
import restApiActions from 'app/flux/restApi/actions';
import { requestStatus } from 'app/flux/restApi/getters';

let _featureId = 0;

const ensureActionType = actionType => {
  if (!actionType) {
    ++_featureId;
    return `TRYING_TO_INIT_FEATURE_${_featureId}`;
  }

  return actionType;
}

export default class FeatureBase {

  initAttemptActionType = '';
    
  constructor(actionType) {
    this.initAttemptActionType = ensureActionType(actionType);
  } 
      
  preload(){
    return $.Deferred().resolve();    
  }
  
  onload() {}
      
  startProcessing() {
    restApiActions.start(this.initAttemptActionType);        
  }

  stopProcessing() {    
    restApiActions.success(this.initAttemptActionType);
  }
    
  isReady(){
    return this._getInitAttempt().isSuccess;
  }

  isProcessing() {
    return this._getInitAttempt().isProcessing;
  }

  isFailed() {
    return this._getInitAttempt().isFailed;
  }

  wasInitialized(){
    const attempt = this._getInitAttempt();        
    return attempt.isFailed || attempt.isProcessing || attempt.isSuccess;
  }

  getIndexRoute(){
    throw Error('not implemented');
  }

  getErrorText() {
    const { message } = this._getInitAttempt();
    return isObject(message) ? message.text : message;              
  }

  getErrorCode(){
    const { message } = this._getInitAttempt();
    return isObject(message) ? message.code : null;
  }

  getIndexComponent(){
    return null;
  }

  handleAccesDenied() {
    this.handleError(new Error('Access Denied'));
  }
  
  handleError(err) {            
    let message = api.getErrorText(err);                
    if (err.status === RestRespCodeEnum.FORBIDDEN) {          
      message = {
        code: RestRespCodeEnum.FORBIDDEN,
        text: message
      }
    }      
    
    restApiActions.fail(this.initAttemptActionType, message);        
  }
    
  withMe(component) {
    return withFeature(this)(component);
  }

  initAttemptGetter(){    
    return requestStatus(this.initAttemptActionType);      
  }
  
  componentDidMount(){     
  }

  _getInitAttempt(){
    return reactor.evaluate(this.initAttemptGetter());
  }
}
