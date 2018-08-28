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

import { Provider } from 'nuclear-js-react-addons';
import reactor from 'app/reactor';
import { setCurrentSiteId } from './../../../flux/currentSite/actions';
import expect from 'expect';
import { $, spyOn } from 'app/__tests__/';
import { makeHelper } from 'app/__tests__/domUtils';
import React from 'react';
import * as ReactDOM from 'react-dom';
import * as fakeData from 'app/__tests__/apiData'
import PodsTab from './../podsTab';
import JobsTab from './../jobsTab';
import NodesTab from './../nodesTab';
import ServicesTab from './../servicesTab';
import DeploymentsTab from './../deploymentsTab';
import k8sServices from 'app/services/k8s';
import * as dateUtils from 'app/lib/dateUtils';
import './../../../index';

const $node = $('<div>');
const helper = makeHelper($node);

describe('app/modules/site/components/k8sPage/main', () => {      
  beforeEach(() => {          
    helper.setup();    
    setCurrentSiteId('siteId');
    spyOn(dateUtils, 'displayK8sAge').andReturn('2 days')
    spyOn(k8sServices, 'getPods').andReturn($.Deferred().resolve(fakeData.k8sPods.items));
    spyOn(k8sServices, 'getJobs').andReturn($.Deferred().resolve(fakeData.k8sJobs.items));
    spyOn(k8sServices, 'getNodes').andReturn($.Deferred().resolve(fakeData.k8sNodes.items));
    spyOn(k8sServices, 'getServices').andReturn($.Deferred().resolve(fakeData.k8sServices.items));
    spyOn(k8sServices, 'getDeployments').andReturn($.Deferred().resolve(fakeData.k8sDeployments.items));
  })

  afterEach(function () {    
    helper.clean();
    reactor.reset();
    expect.restoreSpies();  
  })
        
  it('should render pods', () => {   
    const props = { namespace: 'kube-system' };                              
    render(<PodsTab {...props}/>);    
    expect($node.html()).toBe(podsHtmlSnapshot);
  });        

  it('should render jobs', () => {   
    const props = { namespace: 'kube-system' };                          
    render(<JobsTab {...props}/>);          
    expect($node.html()).toBe(jobsHtmlSnapshot);
  });        

  it('should render nodes', () => {       
    render(<NodesTab/>);        
    expect($node.html()).toBe(nodesHtmlSnapshot);
  });        

  it('should render services', () => {           
    const props = { namespace: 'kube-system' };                              
    render(<ServicesTab {...props}/>);     
    expect($node.html()).toBe(servicesHtmlSnapshot);
  });        

  it('should render deployments', () => {           
    const props = { namespace: 'kube-system' };                              
    render(<DeploymentsTab {...props}/>);     
    expect($node.html()).toBe(deploymentsHtmlSnapshot);
  });        
    
});

const render = cmpnt => {      
  ReactDOM.render((  
  <Provider reactor={reactor}>        
    {cmpnt}
  </Provider>),
  $node[0]);    
}

const podsHtmlSnapshot = `<div data-reactroot="" class="grv-site-k8s-pods"><!-- react-empty: 2 --><table class="table grv-table grv-table-with-details grv-site-k8s-table"><thead class="grv-table-header"><tr><th class="grv-table-cell --col-name">Name</th><th class="grv-table-cell --col-status">Status</th><th class="grv-table-cell --col-containers">Containers</th><th class="grv-table-cell ">Labels</th></tr></thead><tbody><tr><td class="grv-table-cell grv-site-k8s-pods-name"><div style="display: flex; align-items: baseline;"><li class="grv-table-indicator-expand fa fa-chevron-right"></li><div><div class="grv-dropdown-menu"><a class="dropdown-toggle" data-toggle="dropdown" href="#" role="button"><!-- react-text: 18 -->bandwagon-400126467-8t68q<!-- /react-text --><!-- react-text: 19 --> <!-- /react-text --><span class="caret"></span></a><ul class="dropdown-menu multi-level"><li><a>Logs</a></li></ul></div><div><small><!-- react-text: 26 -->host: <!-- /react-text --><!-- react-text: 27 -->172.28.128.7<!-- /react-text --></small></div><small><!-- react-text: 29 -->pod: <!-- /react-text --><!-- react-text: 30 -->10.244.27.4<!-- /react-text --></small></div></div></td><td class="grv-table-cell grv-site-k8s-pods-status"><strong class="text-success">Running</strong></td><td class="grv-table-cell grv-site-k8s-pods-container" rowspan="1"><div class="grv-dropdown-menu"><a class="dropdown-toggle" data-toggle="dropdown" href="#" role="button"><!-- react-text: 36 -->bandwagon<!-- /react-text --><!-- react-text: 37 --> <!-- /react-text --><span class="caret"></span></a><ul class="dropdown-menu multi-level"><div class="m-t-sm p-w-xs m-b-xs text-muted" style="font-size: 11px;"><span>SSH to bandwagon as:</span></div><li class="grv-dropdown-menu-item-login-input"><div class="input-group-sm m-b-xs"><i class="fa fa-terminal m-r"> </i><input class="form-control" placeholder="Enter login name..."></div></li><li class="divider"></li><li><a>Logs</a></li></ul></div></td><td class="grv-table-cell grv-site-k8s-pods-label grv-site-k8s-table-label" rowspan="1"><div><div class="label">app:bandwagon</div></div><div><div class="label">pod-template-hash:400126467</div></div></td></tr><!-- react-empty: 54 --></tbody></table></div>`;
const jobsHtmlSnapshot = `<div data-reactroot="" class="grv-site-k8s-jobs"><!-- react-empty: 2 --><table class="table grv-table grv-table-with-details grv-site-k8s-table"><thead class="grv-table-header"><tr><th class="grv-table-cell --col-name">Name</th><th class="grv-table-cell ">Desired</th><th class="grv-table-cell ">Status</th><th class="grv-table-cell ">Age</th></tr></thead><tbody><tr><td class="grv-table-cell "><div style="display: flex; align-items: baseline;"><li class="grv-table-indicator-expand fa fa-chevron-right"></li><div style="font-size: 14px;">bandwagon-install-3cd80c</div></div></td><td class="grv-table-cell grv-site-k8s-jobs-desired">1</td><td class="grv-table-cell grv-site-k8s-jobs-status"><strong><!-- react-text: 19 --> <!-- /react-text --><div class="text-success"><!-- react-text: 21 -->Succeeded: <!-- /react-text --><!-- react-text: 22 -->1<!-- /react-text --></div></strong><strong><!-- react-text: 24 --> <!-- /react-text --></strong><strong><!-- react-text: 26 --> <!-- /react-text --></strong></td><td class="grv-table-cell ">2 days</td></tr><!-- react-empty: 28 --></tbody></table></div>`;
const nodesHtmlSnapshot = `<div data-reactroot="" class="grv-site-k8s-nodes"><!-- react-empty: 2 --><table class="table grv-table grv-table-with-details grv-site-k8s-table"><thead class="grv-table-header"><tr><th class="grv-table-cell --col-name">Node</th><th class="grv-table-cell --col-status">Status</th><th class="grv-table-cell ">Labels</th><th class="grv-table-cell ">Heartbeat</th></tr></thead><tbody><tr><td class="grv-table-cell "><div style="display: flex; align-items: baseline;"><li class="grv-table-indicator-expand fa fa-chevron-right"></li><div><div style="font-size: 14px;"><!-- react-text: 17 --> <!-- /react-text --><!-- react-text: 18 -->172.28.128.7<!-- /react-text --></div><small><!-- react-text: 20 -->cpu: <!-- /react-text --><!-- react-text: 21 -->2<!-- /react-text --><!-- react-text: 22 -->, <!-- /react-text --></small><small><!-- react-text: 24 -->ram: <!-- /react-text --><!-- react-text: 25 -->2842072Ki<!-- /react-text --></small><br><small><!-- react-text: 28 -->os: <!-- /react-text --><!-- react-text: 29 -->Debian GNU/Linux 8 (jessie)<!-- /react-text --></small><br></div></div></td><td class="grv-table-cell grv-site-k8s-nodes-status"><strong class="text-success">ready</strong></td><td class="grv-table-cell grv-site-k8s-nodes-label" rowspan="1"><div class="grv-site-k8s-table-label"><div class="label">beta.kubernetes.io/arch:amd64</div></div><div class="grv-site-k8s-table-label"><div class="label">beta.kubernetes.io/os:linux</div></div><div class="grv-site-k8s-table-label"><div class="label">gravitational.io/k8s-role:master</div></div><div class="m-t-xs">and more...</div></td><td class="grv-table-cell grv-site-k8s-nodes-heart">8/8/2017 5:04:52 PM</td></tr><!-- react-empty: 42 --></tbody></table></div>`;
const servicesHtmlSnapshot = `<div data-reactroot="" class="grv-site-k8s-services"><!-- react-empty: 2 --><table class="table grv-table grv-table-with-details grv-site-k8s-table"><thead class="grv-table-header"><tr><th class="grv-table-cell --col-name">Name</th><th class="grv-table-cell --col-cluster">Cluster</th><th class="grv-table-cell --col-port">Ports</th><th class="grv-table-cell --col-label">Labels</th></tr></thead><tbody><tr><td class="grv-table-cell "><div style="display: flex; align-items: baseline;"><li class="grv-table-indicator-expand fa fa-chevron-right"></li><div style="font-size: 14px;">kubernetes</div></div></td><td class="grv-table-cell ">10.100.0.1</td><td class="grv-table-cell "><div class="grv-site-k8s-table-label"><div class="label"><!-- react-text: 20 -->TCP:443<!-- /react-text --><li class="fa fa-long-arrow-right m-r-xs m-l-xs"> </li><!-- react-text: 22 -->443<!-- /react-text --></div></div></td><td class="grv-table-cell "><!-- react-text: 24 --> <!-- /react-text --><div class="grv-site-k8s-table-label"><div class="label">component:apiserver</div></div><div class="grv-site-k8s-table-label"><div class="label">provider:kubernetes</div></div><!-- react-text: 29 --> <!-- /react-text --></td></tr><!-- react-empty: 30 --></tbody></table></div>`;
const deploymentsHtmlSnapshot = `<div data-reactroot="" class="grv-site-k8s-deployments"><!-- react-empty: 2 --><table class="table grv-table grv-table-with-details grv-site-k8s-table"><thead class="grv-table-header"><tr><th class="grv-table-cell --col-name">Name</th><th class="grv-table-cell ">Desired</th><th class="grv-table-cell ">Current</th><th class="grv-table-cell ">Up-to-date</th><th class="grv-table-cell ">Available</th><th class="grv-table-cell ">Age</th></tr></thead><tbody><tr><td class="grv-table-cell "><div style="display: flex; align-items: baseline;"><li class="grv-table-indicator-expand fa fa-chevron-right"></li><div style="font-size: 14px;">bandwagon</div></div></td><td class="grv-table-cell ">1</td><td class="grv-table-cell ">1</td><td class="grv-table-cell ">1</td><td class="grv-table-cell ">1</td><td class="grv-table-cell ">2 days</td></tr><!-- react-empty: 23 --></tbody></table></div>`;

