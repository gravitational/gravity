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

import ReactTestUtils from 'react-dom/test-utils';
import * as ReactDOM from 'react-dom';
import expect from 'expect';
import $ from 'jQuery';

export { ReactDOM }

export const makeHelper = node => {
  const $node = $(node);

  return {

    setup() {
      $node.appendTo("body");
    },

    clean() {
      ReactDOM.unmountComponentAtNode($node[0]);
      $(node).remove();
    },

    setText(el, val) {
      ReactTestUtils.Simulate.change(el, { target: { value: val } });
    },

    keyDown(node, key) {
      ReactTestUtils.Simulate.keyDown(node, key);
    },

    keyUp(node, key) {
      ReactTestUtils.Simulate.keyUp(node, key);
    },

    shouldExist(selector){
      expect($node.find(selector).length).toBe(1);
    },

    shouldNotExist(selector){
      expect($node.find(selector).length).toBe(0);
    }
  }
}