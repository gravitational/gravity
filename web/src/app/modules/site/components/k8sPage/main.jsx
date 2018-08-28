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
import { Link } from 'react-router';
import connect from 'app/lib/connect';
import cfg from 'app/config';
import { DropDown } from 'app/components/common/dropDown';
import k8sGetters from './../../flux/k8s/getters';
import * as k8sActions from './../../flux/k8s/actions';
import classnames from 'classnames';
import * as featureFlags from './../../../featureFlags'
import Input from 'app/components/common/input';

class K8sPage extends React.Component {

  static contextTypes = {
    router: React.PropTypes.object.isRequired
  }

  constructor(props){
    super(props);
    const { siteId } = this.props.routeParams;
    const tabItems = [
      { isIndex: true, to: cfg.getSiteK8sRoute(siteId), title: "Nodes" },
      { to: cfg.getSiteK8sPodsRoute(siteId), title: "Pods" },
      { to: cfg.getSiteK8sServicesRoute(siteId), title: "Services" },
      { to: cfg.getSiteK8sJobsRoute(siteId), title: "Jobs" },
      { to: cfg.getSiteK8sDaemonsRoute(siteId), title: "Daemon Sets" },
      { to: cfg.getSiteK8sDeploymentsRoute(siteId), title: "Deployments" },      
    ];

    this.state = { 
      tabItems 
    };
  }
  
  renderTabHeaderItem(item, key){
    const { to, isIndex, title } = item;
    const className = this.context.router.isActive(to, isIndex) ? "active" : "";
    return (
      <li key={key} className={className}>
        <Link to={to}>
          <h4 className="nav-label m-b-xxs">
            {title}
          </h4>
        </Link>
      </li>
    )
  }

  isSearchAvailable(){
    const { router } = this.context;
    const { siteId } = this.props.routeParams;
    return router.isActive(cfg.getSiteK8sPodsRoute(siteId), true);
  }

  isNamespaceAvailable(){
    const { router } = this.context;
    const { siteId } = this.props.routeParams;
    return !router.isActive(cfg.getSiteK8sNodesRoute(siteId), true);
  }

  renderNamespaceSelector(){
    const { namespaces, k8s } = this.props;
    const { curNamespace } = k8s;
    const namespaceOptions = namespaces.map(item => ({
      value: item,
      label: item
    }));

    return (
      <div className="grv-site-k8s-namespace m-l-sm">
        <DropDown
          right={true}
          size="sm"
          onChange={k8sActions.setCurNamespace}
          value={curNamespace}
          options={namespaceOptions}
        />
      </div>
    )
  }

  render() {            
    const { curNamespace, searchValue } = this.props.k8s;
    const $headerItems = this.state.tabItems.map(this.renderTabHeaderItem.bind(this));
    const childProps = {
      namespace: curNamespace,
      searchValue,
      monitoringEnabled: featureFlags.siteMonitoring()
    };
                
    const isSearchAvailable = this.isSearchAvailable();        
    const toolbarClass = classnames('grv-site-k8s-toolbar', { '--long' : isSearchAvailable });  
    const $namespaceSelector = this.isNamespaceAvailable() ? this.renderNamespaceSelector() : null;
    const $content =  React.cloneElement(this.props.children, childProps);
    
    return (
      <div className="grv-site-k8s grv-page">
        <div className="tabs-container">
          <div className={toolbarClass}>
            { isSearchAvailable && 
              <div className="input-group input-group-sm">
                <Input className="form-control" 
                  placeholder="Search..."
                  autoFocus 
                  value={searchValue} 
                  onChange={k8sActions.setSearchValue} />
              </div>
            }            
            {$namespaceSelector}
          </div>
          <ul className="nav nav-tabs">
            {$headerItems}
          </ul>
          <div className="tab-content">
            <div className="tab-pane active">
              <div className="panel-body">
                {$content}
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }
}

const mapStateToProps = () => ({
  namespaces: k8sGetters.namespaces, 
  k8s: k8sGetters.k8s
})

export default connect(mapStateToProps)(K8sPage);
