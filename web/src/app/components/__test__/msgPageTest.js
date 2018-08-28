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
import * as ReactDOM from 'react-dom';
import { Router, createMemoryHistory } from 'react-router';
import expect from 'expect';
import DocumentTitle from 'app/components/common/documentTitle';
import cfg from 'app/config';
import * as Messages from 'app/components/msgPage'
import { $ } from 'app/__tests__/';
import { makeHelper } from 'app/__tests__/domUtils';

const $node = $('<div>');
const helper = makeHelper($node);

let rootRoutes = [  
  {
    component: DocumentTitle,
    childRoutes: [  
      { path: cfg.routes.errorPage, component: Messages.ErrorPage },
      { path: cfg.routes.infoPage, component: Messages.InfoPage }    
    ]
  }
]

describe('components/msgPage', function () {
    
  const history = new createMemoryHistory();   

  beforeEach(()=>{
    helper.setup()
  });

  afterEach(() => {    
    helper.clean();    
  })
  
  it('should render default error', () => {                                     
    history.push('/web/msg/error')
    render();    
    expectHeaderText(Messages.MSG_ERROR_DEFAULT)
    expectDetailsText('')
  });
        
  it('should render login failed', () => {                                         
    history.push('/web/msg/error/login_failed?details=test')
    render();
    expectHeaderText(Messages.MSG_ERROR_LOGIN_FAILED)
    expectDetailsText('test')
  });

  it('should render expired link', () => {                                         
    history.push('/web/msg/error/expired_link')
    render();
    expectHeaderText(Messages.MSG_ERROR_LINK_EXPIRED)    
  });

  it('should render invalid user', () => {                                         
    history.push('/web/msg/error/invalid_user')
    render();
    expectHeaderText(Messages.MSG_ERROR_INVALID_USER)    
  });

  it('should render not found', () => {                                         
    history.push('/web/msg/error/not_found')
    render();
    expectHeaderText(Messages.MSG_ERROR_NOT_FOUND)        
  });

  it('should render login succesfull', () => {                                         
    history.push('/web/msg/info/login_success')
    render();
    expectHeaderText(Messages.MSG_INFO_LOGIN_SUCCESS)
  });

  it('should render site offline', () => {                                                 
    ReactDOM.render(( <Messages.SiteOffline siteId="hello"/> ) , $node[0]);        
    expect($node.find('.grv-msg-page h3:first').text())
      .toEqual('This cluster is not available from Telekube.')
  });

  it('should render site is scheduled for uninstall', () => {                                                 
    ReactDOM.render(( <Messages.SiteUninstall siteId="hello"/> ) , $node[0]);        
    expect($node.find('.grv-msg-page h3:first').text())
      .toEqual('This cluster has been scheduled for deletion.')
  });

  const expectDetailsText = text => {
    expect($node.find('.grv-msg-page small').text()).toEqual(text)
  }

  const expectHeaderText = text => {
    expect($node.find('.grv-msg-page h1:first').text()).toEqual(text)
  }

  const render = () => {    
    ReactDOM.render(( <Router history={history} routes={rootRoutes} /> ) , $node[0]);  
  }

});

