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
import webSockets from 'app/services/webSockets';

const DEFAULT_INTERVAL = 3000; // every 3 sec

export default class DataProvider extends React.Component {
  
  _timerId = null;
  _request = null;

  constructor(props) {
    super(props);
    this._intervalTime = props.time || DEFAULT_INTERVAL;
  }

  fetch() {
    // do not refetch if still in progress
    if (this._request) {      
      return;
    }

    this._request = this.props.onFetch()
      .always(() => {
        this._request = null;
      })
  }

  componentDidMount() {        
    this.fetch();
    this._timerId = setInterval(this.fetch.bind(this), this._intervalTime);
  }

  componentWillUnmount(){
    clearInterval(this._timerId);
    if (this._request && this._request.abort) {
      this._request.abort();
    }  
  }

  render() {
    return null;
  }
}

export class SiteOperationLogProvider extends React.Component {

  static propTypes = {
   siteId: React.PropTypes.string.isRequired,
   opId: React.PropTypes.string.isRequired,
   onLoading: React.PropTypes.func,
   onError: React.PropTypes.func,
   onData: React.PropTypes.func
  }

  constructor(props) {
    super(props);
    this.socket = null;    
  }
  
  componentWillReceiveProps(nextProps){
    let {siteId, opId} = this.props;
    if(nextProps.opId !== opId){
      this.connect(siteId, nextProps.opId);
    }
  }

  componentDidMount() {
    let {siteId, opId} = this.props;
    this.connect(siteId, opId);
  }

  componentWillUnmount(){
    this.disconnect();
  }

  disconnect(){
    if(this.socket){
      this.socket.close();
    }
  }

  onLoading(value){
    if(this.props.onLoading){
      this.props.onLoading(value);
    }
  }

  onError(err){
    if(this.props.onError){
      this.props.onError(err);
    }
  }

  onData(data){
    if(this.props.onData){
      this.props.onData(data.trim() + '\n');
    }
  }

  connect(siteId, opId){    
    this.disconnect();    
    this.onLoading(true);

    this.socket = webSockets.createLogStreamer(siteId, opId);
    this.socket.onopen = () => { this.onLoading(false); };
    this.socket.onerror = () => { this.onError(); }
    this.socket.onclose = () => { };
    this.socket.onmessage = e => { this.onData(e.data); };
  }

  render() {
     return null;
  }
}