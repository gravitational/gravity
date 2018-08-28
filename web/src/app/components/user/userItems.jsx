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
import classnames from 'classnames';
import { AuthProviderTypeEnum } from 'app/services/enums';

const guessProviderBtnClass = (name, type) => {  
  name = name.toLowerCase();

  if (name.indexOf('microsoft') !== -1) {
    return 'btn-microsoft';    
  }

  if (name.indexOf('bitbucket') !== -1) {
    return 'btn-bitbucket';  
  }
  
  if (name.indexOf('google') !== -1) {
    return 'btn-google';
  }

  if (name.indexOf('github') !== -1 || type === AuthProviderTypeEnum.GITHUB ) {
    return 'btn-github';
  }

  if (type === AuthProviderTypeEnum.OIDC) {
    return 'btn-openid';
  }

  return '--unknown';   
}


const SsoBtnList = ({providers, className, prefixText, isDisabled, onClick}) => {
  let $btns = providers.map((item, index) => {
    let { name, type, displayName } = item;    
    displayName = displayName || name;
    const title = `${prefixText} ${displayName}`
    const providerBtnClass = guessProviderBtnClass(displayName, type);

    let btnClass = `btn grv-user-btn-sso full-width ${providerBtnClass}`;
    return (
      <button key={index}
        disabled={isDisabled}
        className={btnClass}
        onClick={e => { e.preventDefault(); onClick(item) }}>      
        <div className="--sso-icon">
          <span className="fa"/>
        </div>
        <span>{title}</span>      
      </button>              
      )
  })

 if ($btns.length === 0) {
    return (
      <h4> You have no SSO providers configured </h4>
    )
  }
  
  return (
    <div className={classnames(className)}> {$btns} </div>
  )
}

const Form = React.createClass({

  refCb(e) {    
    if (this.props.refCb) {
      this.props.refCb(e);
    }
  },

  onSubmit(e) {    
    e.preventDefault();
    return false;
  },

  render() {    
    let { className='', style = {}, children } = this.props;
    return (
      <form ref={this.refCb}
        className={className}
        style={style}
        onSubmit={this.onSubmit}>
        {children}
      </form>
    );
  }
});

export { SsoBtnList, Form }
