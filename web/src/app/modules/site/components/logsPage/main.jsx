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
import { download } from  'app/services/downloader';
import cfg from 'app/config';
import LogViewer from 'app/components/logViewer';
import userGetters from 'app/flux/user/getters';
import sitePodGetters from './../../flux/k8sPods/getters';
import connect from 'app/lib/connect';
import { SiteLogAggregatorProvider } from './../dataProviders';
import QueryEditor from './queryEditor';

class SiteLogs extends React.Component {

  state = {    
    shouldRefresh: false
  }
  
  onKeyPress = e => {
    if (e.key === 'Enter') {
      this.props.onNewQuery(e.target.value);
    }        
  }
  
  onRefresh = () => {    
    this.setState({ shouldRefresh: true }, () => {
      this.setState({ shouldRefresh: false });      
    });
  }

  onSearch = query => {
    this.props.onNewQuery && this.props.onNewQuery(query);
  }
    
  onDownload = () => {
    download(this.props.downloadUrl)
  }

  render() {
    const { shouldRefresh } = this.state;
    const { query = '', queryUrl, suggestions } = this.props;                
    
    return (
      <div className="grv-site-logs grv-page">
        <div className="grv-site-logs-header">
          <div>
            <h3 className="grv-site-header-size no-margins">Logs</h3>
          </div>
          <div className="grv-site-logs-header-controls">            
            <QueryEditor suggestions={suggestions} query={query} onChange={this.onSearch}/>                            
            <button              
              onClick={this.onRefresh}
              className="btn btn-sm btn-white">
              <i className="fa fa-refresh m-r-xs" aria-hidden="true"/>
              <span>Refresh</span>
            </button>                        
          </div>          
        </div>
        <LogViewer
          ref={ e => this.logViewerRef = e}
          className="grv-site-logviewer"  
          autoScroll={true}            
          provider={
            <SiteLogAggregatorProvider
              forceRefresh={shouldRefresh}              
              queryUrl={queryUrl} />
          }          
        />        
      </div>
    )
  }
}

class SiteLogsContainer extends React.Component {
  
  static contextTypes = {
    router: React.PropTypes.object.isRequired
  }

  onNewQuery = query => {
    const loc = this.props.location;
    loc.query.query = query;
    this.context.router.push(loc);
  }

  render(){
    const { query='' } = this.props.location.query;
    const { siteId } = this.props.params;
    const suggestions = this.props.autoCompleteOptions;
    const user = reactor.evaluateToJS(userGetters.user);
    const queryUrl = cfg.getSiteLogAggregatorUrl(siteId, user.accountId, query);
    const downloadUrl = cfg.getSiteDownloadLogUrl(siteId, user.accountId, query);
    
    const props = {
      suggestions,
      query,
      queryUrl,
      downloadUrl,
      onNewQuery: this.onNewQuery
    };

    return <SiteLogs {...props}/>
  }
}

const mapFluxToState = () => ({
  autoCompleteOptions: sitePodGetters.autoCompleteOptions
})

export default connect(mapFluxToState)(SiteLogsContainer);