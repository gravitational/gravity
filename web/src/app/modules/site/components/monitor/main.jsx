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
import cfg from 'app/config';
import $ from 'jQuery';
import api from 'app/services/api';
import Indicator from 'app/components/common/indicator';
import history from 'app/services/history';
import grafanaCss from '!raw-loader!./grafana.css';

const GRAFANA_CSS_OVERRIDES = `<style>${grafanaCss}</style>`;

class Monitor extends React.Component {

  state = {
    isInitializing: true,
    isLoading: false,
    isError: false
  }

  initGrafana() {
    let { siteId, splat } = this.props.params;
    let { search } = this.props.location;

    let contextUrl = cfg.getSiteGrafanaContextUrl(siteId)
    return api.put(contextUrl).then(json  => {
      if( !json  || !json.url){
        return $.Deferred().reject(new Error("Could not resolve grafana endpoints"))
      }

      return ensureGrafanaUrl(json.url, splat, search)
    })
  }

  componentDidMount(){
    this.initGrafana().done(url => {
      this.url = url;
      this.setState({
        isInitializing: false,
        isLoading: true
      }, () =>{
        this._tweakGrafana();
      });
    })
    .fail(err => {
      var errorText = api.getErrorText(err);
      this.setState({
        isError: true,
        errorText
      })
    })
  }

  render() {
    const { isLoading, isInitializing, isError, errorText } = this.state;
    let $indicator = null;

    if (isError){
      $indicator = <ErrorIndicator message={errorText}/>
    }else if (isLoading || isInitializing){
      $indicator = <Indicator enabled={true} type={'bounce'}/>
    }

    // show grafana iframe only when initialized
    const showGrafana = !isInitializing && !isError;

    return (
      <div className="grv-site-monitor m-t-sm m-b-sm">
        {$indicator}
        {showGrafana &&
          <iframe className="grv-site-monitor-grafana"
            src={this.url}
            frameBorder="0" />
        }
      </div>
    )
  }

  _tweakGrafana(){
    let $iframe = $('.grv-site-monitor iframe');
    $iframe.load(() => {
      this.setState({ isLoading: false });
      $iframe.contents()
        .find('head')
        .append($(GRAFANA_CSS_OVERRIDES))

      $iframe.addClass("--loaded");
    })
  }
}

const errorIndicatorStyle = {
  'zIndex': '1',
  'flex': '1',
  'justifyContent': 'center',
  'display': 'flex',
  'alignItems': 'center'
}

const ErrorIndicator = ({message}) => (
  <div style={errorIndicatorStyle}>
    <i className="fa fa-exclamation-triangle fa-3x text-warning"></i>
    <div className="m-l">
      <strong>Error</strong>
      <div style={{maxWidth:"200px", wordBreak:"break-all"}}><small>{message}</small></div>
    </div>
  </div>
)

const ensureGrafanaUrl = (baseUrl, dashboard, query) => {
  let url = `${baseUrl}/${dashboard}/${query}`;
  let grafanaDefaultDashboardUrl = cfg.getSiteDefaultDashboard();

  url = url.replace(/\/\/+/g, '/');

  // if empty query, use default dashboard if provided
  if (url === baseUrl+'/' && grafanaDefaultDashboardUrl) {
    url = `${baseUrl}/${grafanaDefaultDashboardUrl}`;
  }

  return history.ensureBaseUrl(url);
}

export default Monitor;
