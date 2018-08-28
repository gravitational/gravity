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

import { Store, toImmutable } from 'nuclear-js';
import {
  REST_API_START,
  REST_API_SUCCESS,
  REST_API_FAIL,
  REST_API_CLEAR } from './actionTypes';

export default Store({

  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(REST_API_START, start);
    this.on(REST_API_FAIL, fail);
    this.on(REST_API_SUCCESS, success);
    this.on(REST_API_CLEAR, clear);
  }
})

function start(state, request){
  return state.set(request.type, toImmutable({isProcessing: true}));
}

function fail(state, request){
  return state.set(request.type, toImmutable({isFailed: true, message: request.message}));
}

function success(state, request){
  return state.set(request.type, toImmutable({isSuccess: true, message: request.message}));
}

function clear(state, request){
  return state.set(request.type, toImmutable(
    {
      isProcessing: false,
      isFailed: false,
      isSuccess: false,
      message: undefined
    }));  
}