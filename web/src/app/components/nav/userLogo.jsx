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
import getters from 'app/flux/user/getters';
import * as actions from 'app/flux/user/actions';

const UserLogo = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return { user: getters.user }
  },

  renderMenuItem(item, key) {    
    let { to, text, enabled=true } = item;

    if (!enabled){
      return (
      <li key={key} className="disabled">
        <a>{text}</a>
      </li>
      )
    }

    return (
      <li key={key}>
        <a href={to}>{text}</a>
      </li>
    )
  },

  render() {
    let { userId } = this.state.user;
    let { menuItems = [] } = this.props;
    let $menuItems = menuItems.map(this.renderMenuItem);

    return (
      <div className="grv-nav-user">
        <a href="#" className="dropdown-toggle" data-toggle="dropdown" role="button">
          <span className="m-r-xs">
            {userId}
          </span>
          <span className="caret"></span>
        </a>
        <ul className="dropdown-menu dropdown-menu-right pull-right">          
          {$menuItems}                    
          <li>
            <a href="#" onClick={actions.logout}>              
              Log out
            </a>
          </li>
        </ul>
      </div>
    )
  }
});

export default UserLogo;
