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
import * as actions from './../../flux/masterConsole/actions';
import cfg from 'app/config';
import localStorage from 'app/services/localStorage';
import XTerm from 'app/lib/term/terminal';
import TtyAddressResolver from 'app/lib/term/ttyAddressResolver';

class TerminalContainer extends React.Component {

  static contextTypes = {
    isTabActive: React.PropTypes.bool,
    isParentVisible: React.PropTypes.bool
  }

  static propTypes = {
    clusterName: React.PropTypes.string.isRequired,
    login: React.PropTypes.string.isRequired,
    title: React.PropTypes.string.isRequired,
    serverId: React.PropTypes.string.isRequired
  }

  state = {
    sid: null,
    isConnected: false
  }

  updateState(newState){
    this.setState({...newState});
  }

  componentDidMount(){
    if(this.shouldConnect()){
      this.makeConnection();
    }
  }

  componentDidUpdate(){
    if(this.shouldConnect()){
      this.makeConnection();
    }
  }

  shouldConnect(){
    const { sid, isConnected } = this.state;
    const { isTabActive, isParentVisible } = this.context;
    return !isConnected && isTabActive && isParentVisible && !sid;
  }

  makeConnection(){
    const { login, clusterName } = this.props;
    actions.createNewSession(login, clusterName)
      .done(json=>{
        const { id } = json.session;
        this.updateState({sid: id})
      })
      .fail(()=>{
        this.updateState({isError: true});
      })
  }

  render(){
    const { sid } = this.state;
    const props = {
      sid,
      ...this.props
    }

    return (
      <div className="grv-site-mconsole-terminal">
        { sid ? <TerminalControl {...props}/> : null }
      </div>
    )
  }
}

class TerminalControl extends React.Component {

  componentDidMount() {
    const { serverId, login, pod, sid, clusterName } = this.props;
    const accessToken = localStorage.getAccessToken();
    const addressResolver = new TtyAddressResolver({
      sid,
      login,
      ttyUrl: pod ? cfg.api.ttyWsK8sPodAddr : cfg.api.ttyWsAddr,
      cluster: clusterName,
      token: accessToken,
      getTarget(){
        return pod ? { pod } : { server_id : serverId }
      }
    })

    this.terminal = new XTerm({
      el: this.refs.container,
      addressResolver
    });

    this.terminal.open();
  }

  componentWillUnmount() {
    this.terminal.destroy();
  }

  shouldComponentUpdate() {
    return false;
  }

  render() {
    return (<div ref="container"/>);
  }
}

export default TerminalContainer;
