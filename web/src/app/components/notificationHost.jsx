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

import React from 'react';
import reactor from 'app/reactor';
import notifGetters from 'app/flux/notifications/getters';
import { escape } from 'lodash'
import toastr from 'toastr';

toastr.options = {
  'closeButton': true,
  'preventDuplicates': true,
  'hideDuration': '100'
}

const NotificationHost = React.createClass({

  componentDidMount() {
    reactor.observe(notifGetters.lastMessage, this.update)
  },

  componentWillUnmount() {
    reactor.unobserve(notifGetters.lastMessage, this.update);
  },

  update(msg) {
    if (msg) {
      let { text, title, escapeHtml=true } = msg;          
      if (escapeHtml) {
        text = escape(text);
        title = escape(title);
      }
            
      if (msg.isError) {
        toastr.error(text, title);
      } else if (msg.isWarning) {
        toastr.warning(text, title);
      } else if (msg.isSuccess) {
        toastr.success(text, title);
      } else {
        toastr.info(text, title);
      }
    }
  },

  render() {
   return null;
  }

});

export default NotificationHost;
