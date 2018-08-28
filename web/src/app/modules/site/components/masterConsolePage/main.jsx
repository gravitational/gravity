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
import * as actions from './../../flux/masterConsole/actions';
import getters from './../../flux/masterConsole/getters';
import classnames from 'classnames';
import $ from 'jQuery';
import {TabItem, ServerTabs} from './tabs';
import TerminalContainer from './terminalContainer';

const MasterConsole = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      masterConsole: getters.masterConsole
    }
  },

  childContextTypes: {
    isParentVisible: React.PropTypes.bool
  },

  getChildContext: function() {
    let {isVisible} = this.state.masterConsole;
    return {isParentVisible: isVisible};
  },

  componentDidMount(){
    actions.initMasterConsole();
  },

  componentDidUpdate(){
    let {isVisible} = this.state.masterConsole;

    if(isVisible){
      $('.grv-site-mconsole .terminal').focus();
    }
  },

  onTabClick(index){
    let {terminals} =  this.state.masterConsole;
    if(terminals[index]){
      actions.setActiveTerminal(terminals[index].key);
    }
  },

  onTabClose(index) {
    let {terminals} =  this.state.masterConsole;
    if(terminals[index]){
      actions.removeTerminal(terminals[index].key);
    }     
  },

  renderTermTabItem(terminal){
    let {title, key} = terminal;
    return (
      <TabItem key={key} title={title}>
        <TerminalContainer {...terminal}/>
      </TabItem>
    );
  },

  render() {
    let {terminals, isVisible, activeTerminal, isInitialized} =  this.state.masterConsole;
    let $termTabItems = terminals.map(this.renderTermTabItem);
    let activeTabIndex = 0;
    
    for (let i = 0; i < terminals.length; i++) {
      if (terminals[i].key === activeTerminal) {
        activeTabIndex = i;
        break;
      }
    }

    let className = classnames('grv-site-mconsole m-t-sm m-b-sm', {
        'hidden' : !isVisible
      });

    return (
      <div className={className}>
        {isInitialized ?
          <ServerTabs
            value={activeTabIndex}
            onTabClick={this.onTabClick}
            onTabClose={this.onTabClose}>
            {$termTabItems}
          </ServerTabs> : null
        }
      </div>
    )
  }
});

const MasterConsoleActivator = React.createClass({

  componentDidMount(){
    actions.showTerminal()
  },

  componentWillUnmount(){
    actions.hideTerminal();
  },

  render(){
    return null;
  }
});

export default MasterConsole;

export {
  MasterConsoleActivator
}
