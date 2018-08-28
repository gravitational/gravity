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

import expect from 'expect';
import $ from 'jQuery';

export const mock = api => {
  let ajaxGetMap = {};
  let ajaxPutMap = {};

  expect
    .spyOn(api, 'put')
    .andCall(url => {
      if (ajaxPutMap[url] !== undefined) {
        return ajaxPutMap[url]();
      }
      throw Error(`no mock to handle PUT: ${url} `)
    })

  expect
    .spyOn(api, 'get')
    .andCall(url => {
      if (ajaxGetMap[url] !== undefined) {
        return ajaxGetMap[url]();
      }

      throw Error(`no mock to handle GET: ${url} `)
    })

  return {

    get(url) {
      return {
        andResolve(value) {
          ajaxGetMap[url] = () => $
            .Deferred()
            .resolve(value);
        },

        andReject(value) {
          ajaxGetMap[url] = () => $
            .Deferred()
            .reject(value);
        },

        andReturnPromise(value) {
          ajaxGetMap[url] = () => value;
        }
      }
    },

    put(url) {
      return {
        andResolve(value) {
          ajaxPutMap[url] = () => $
            .Deferred()
            .resolve(value);
        },

        andReject(value) {
          ajaxPutMap[url] = () => $
            .Deferred()
            .reject(value);
        },

        andReturnPromise(value) {
          ajaxPutMap[url] = () => value;
        }
      }
    }

  }
}
