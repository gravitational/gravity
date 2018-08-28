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
import * as actions from './../flux/actions';
import Logger from 'app/lib/logger';
import api from 'app/services/api';
import AjaxPoller from 'app/components/dataProviders'
import { fetchServers } from './../flux/servers/actions';
import { fetchOpProgress } from './../flux/currentSite/actions';

const logger = Logger.create('modules/site/components/siteLogAggregatorProvider');
const POLL_INTERVAL = 3000;
const SITE_SERVERS_POLL_INTERVAL = 6000;

export const SiteOpProgressProvider = React.createClass({

  propTypes: {
   opId: React.PropTypes.string.isRequired
  },
  
  fetchProgress(){
    return fetchOpProgress(this.props.opId);
  },
  
  render() {
    return (    
      <AjaxPoller time={POLL_INTERVAL} onFetch={this.fetchProgress} />
    )  
  }
});

export const SiteOpProvider = () => (
  <AjaxPoller
    time={POLL_INTERVAL}
    onFetch={actions.fetchSiteOps}      
  />
)

export const SiteProvider = () => (
  <AjaxPoller
    time={POLL_INTERVAL}
    onFetch={actions.fetchSite}      
  />
)

export const SiteServerProvider = () => (
  <AjaxPoller
    time={SITE_SERVERS_POLL_INTERVAL}
    onFetch={fetchServers}
  />
)
    
export class SiteLogAggregatorProvider extends React.Component {    

  static propTypes = {    
    forceRefresh: React.PropTypes.bool, 
    queryUrl: React.PropTypes.string.isRequired,
    onLoading: React.PropTypes.func,
    onError: React.PropTypes.func,
    onData: React.PropTypes.func
  }

  constructor(props) {
    super(props)
    this._request = null;
  }
  
  componentWillReceiveProps(nextProps) {    
    let { queryUrl, forceRefresh } = this.props;    
    if(forceRefresh || nextProps.queryUrl !== queryUrl){
      this.fetch(nextProps.queryUrl);
    }
  }

  componentDidMount() {    
    this.fetch(this.props.queryUrl);
  }

  componentWillUnmount(){
    this.rejectCurrentRequest();
  }

  rejectCurrentRequest() {
    if (this._request) {
      this._request.abort();
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

  onData(data) {
    if (!this.props.onData) {
      return;
    }

    try {
      let parsedData = [];      
      data = data || [];          
      data.forEach(item => {
        item = JSON.parse(item);
        if (item.type === 'data') {
          let payload = item.payload || '';
          parsedData.push(payload.trim());    
        }
      });
        
      if (parsedData.length === 0) {
        parsedData.push('No results found')
      }

      this.props.onData(parsedData.join('\n'));            
    }catch(err){
      logger.error('Failed to deserialize', err);
    }
  }

  fetch(queryUrl) {    
    if (queryUrl) {
      queryUrl = queryUrl.trim();
    }
    
    this.rejectCurrentRequest();

    this.onLoading(true);

    this._request = api.get(queryUrl)
      .done(data => {
        this.onLoading(false);
        this.onData(data);
      })
      .fail(err => {        
        if (err.state && err.state() === 'rejected'){
          return;
        }           
        
        let msg = api.getErrorText(err);
        this.onError(msg);
      });          
  }

  render() {
    return null;
  }
}