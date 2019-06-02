/*
Copyright 2019 Gravitational, Inc.

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

import api from './api';
import Events from 'events';

var EventEmitter = Events.EventEmitter;

class Uploader extends EventEmitter {

  constructor(url){
    super();
    this._xhr = new XMLHttpRequest();
    this._url = url;
  }

  abort(){
    this._xhr.abort();
  }

  start(data = {}){
    let xhr = this._xhr;
    let fd = new FormData();
    let self = this;

    Object.getOwnPropertyNames(data).forEach(key => {
      fd.append(key, data[key]);
    })

    return api.ajax({
      url: this._url,
      type: 'PUT',
      data: fd,
      cache : false,
      processData: false,
      contentType: false,
      xhr() {
        xhr.upload.addEventListener('progress', e => {
          if (e.lengthComputable) {
            let progressVal = Math.round((e.loaded/e.total)*100);
            self.emit('progress', progressVal);
          }
        }, false);

        return xhr;
      }
    })
    .done(json => {
      self.emit('completed', json);
    })
    .fail(err =>{
      self.emit('failed', err.message);
    })
  }
}

export {
  Uploader
}
