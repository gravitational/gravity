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

import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import * as AT from './actionTypes';

class DialogsRec extends Record({  
  resourceToDelete: null
}){
    
  setResourceToDelete(item){
    return this.set('resourceToDelete', item);
  }

  getResourceToDelete(){
    return this.get('resourceToDelete');
  }  
}

export default Store({
  getInitialState() {
    return new DialogsRec();
  },

  initialize() {    
    this.on(AT.SET_RES_TO_DELETE, (state, item) => state.setResourceToDelete(item))        
  }
});
