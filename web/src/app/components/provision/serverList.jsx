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
import opAgentGetters from 'app/flux/opAgent/getters';
import opAgentActions from 'app/flux/opAgent/actions';
import {ServerVarEnums} from 'app/services/enums';
import { keyBy, merge, values } from 'lodash';

import DockerVariable from './variables/docker';
import InterfaceVariable from './variables/interface';
import MountVariable from './variables/mount';
import {ServerListLabel} from './items';

const ServerList =  React.createClass({

  propTypes: {
   onChange: React.PropTypes.func.isRequired
  },

  getInitialState(){
    /*
      {
        hostNameA: {
         vars...
        },
        hostNameB: {
         vars...
        }
      }
    */

    let state = keyBy(this.props.servers, item => item.hostName);
    return state;
  },

  componentDidUpdate(){
    setTimeout(this.onChange, 0);
  },

  componentWillReceiveProps(nextProps) {
    let {servers} = nextProps;
    servers = servers.map(item=> merge(item, this.state[item.hostName]));
    this.state = keyBy(servers, item => item.hostName);
  },

  setVariableValue(value, variable){
    variable.value = value;
    this.setState(this.state);
  },

  onChange(){
    if(this.props.onChange){
      let serverArray = values(this.state);
      this.props.onChange(serverArray);
    }
  },

  renderVars(variable, key){    
    let _onChange = (value) =>{
      variable.value = value;
      this.setState(this.state);
      this.onChange()
    }

    switch (variable.type) {
      case ServerVarEnums.INTERFACE:
        return <InterfaceVariable key={key} {...variable} onChange={_onChange}/>
      case ServerVarEnums.MOUNT:
        return <MountVariable key={key} {...variable} onChange={_onChange}/>
      case ServerVarEnums.DOCKER_DISK:
        return <DockerVariable key={key} {...variable} onChange={_onChange}/>
    }

    return null;
  },

  renderServer(server, key){
    let { vars, hostName } = server;
    let $variables = vars.map(this.renderVars);

    return (
      <div className="row grv-provision-req-server" key={key+hostName}>
        <div className="col-sm-12">
          <div className="hr-line-dashed"></div>
        </div>
        <div className="col-sm-12">
          <div style={ServerList.style.server}>
            <div style={{minWidth: '120px'}}>
              <ServerListLabel text="Host name"/>
              <div className="grv-provision-req-server-hostname">
                <span style={{wordWrap: "break-word"}} className="m-r-xs">{hostName}</span>
              </div>
            </div>
            <div>
              <div className="grv-provision-req-server-inputs">
                {$variables}
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  },

  render(){
    let servers = values(this.state);
    let $serverItems = servers.map(this.renderServer);

    return (
      <div>
        {$serverItems}
      </div>
    );
  }
});

ServerList.style = {
  server: {
    display: 'flex',
    justifyContent: 'space-between'
  }
}

var ServerListObserver = React.createClass({
  mixins: [reactor.ReactMixin],

  getDataBindings() {
    let { serverRole, opId } = this.props;
    this.serverRole = serverRole;
    return {
      servers: opAgentGetters.serverVarsByRole(opId, serverRole)
    }
  },

  componentWillUnmount(){
    opAgentActions.clearAgentServerVars(this.serverRole);
  },

  onChange(serverVars){
    opAgentActions.setAgentServerVars(this.serverRole, serverVars)
  },

  render(){
    return (
      <ServerList
        onChange={this.onChange}
        servers={this.state.servers}
      />
    );
  }
});

export default ServerListObserver;