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

import $ from 'jQuery';
import React from 'react';
import Button from 'app/components/common/button';
import { isObject } from 'lodash';
import { RestRespCodeEnum } from 'app/services/enums';
import cfg from 'app/config';

const Warning = ({data}) => {      
  let $content = null;
  if (isObject(data) && data.code === RestRespCodeEnum.FORBIDDEN){            
    let helpLink = cfg.modules.installer.iamPermissionsHelpLink;
    $content = (
      <div>
        <strong>The following IAM permissions are missing:</strong>
        <div>{data.text}</div><br/>
        <a target="_blank" href={helpLink}>click here for more info</a>
      </div>
    )
  } else {
    $content = (
      <div>
        <strong>Cannot continue</strong>
        <div>{data}</div>
      </div>
    )
  }
  
  return (
    <div className="grv-installer-attemp-message">
      <i className="fa fa-minus-circle --warning" aria-hidden="true"></i>
      <div className="grv-installer-attemp-message-text">        
        {$content}
      </div>
    </div>
  );
};

class Footer extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.onClick = this.onClick.bind(this);
  }
    
  onClick(fn) {
    let $forms = $('form');
    let isValid = true;
    for(let i = 0; i < $forms.length; i++){
      $forms.eq(i).validate().settings.ignore = [];
      isValid = $forms.eq(i).valid() && isValid;
    }

    if (isValid) {
      fn();        
    }
  }
      
  renderAttempMessage(attemp) {      
    let {isFailed, message} = attemp;    
    let $msg = null;  
    if (isFailed) {      
      $msg = <Warning data={message}/>  
    }  
    return ( <div> {$msg} </div> )      
  }

  renderPrimaryBtn(text, onClickFn, isProcessing, isDisabled) {        
    return (      
      <Button className="grv-installer-footer-primary-btn grv-installer-btn-new-site btn-primary btn-block"
        key="footer-primary-btn"
        onClick={ () => this.onClick(onClickFn) }
        isDisabled={isDisabled}
        isProcessing={isProcessing}>
        <span>{text}</span>
      </Button>        
    )                      
  }

  /* 
  * allows to override default number of buttons (default is 'Continue')
  */
  renderBtnGroup($btns) {        
    // provide default behavior
    if (!$btns) {       
      let { text, attemp, onClick } = this.props;
      let { isProcessing } = attemp;
      $btns = [this.renderPrimaryBtn(text, onClick, isProcessing)];
    } 
    
    return (
      <div style={{ display: "flex", alignSelf: "flex-start" }}>
        {$btns}
      </div>
    )
  }

  render() {
    let { attemp } = this.props;          
    let $msg = this.renderAttempMessage(attemp)  
    let $btnGroup = this.renderBtnGroup();    
    return (      
      <div className="grv-installer-footer m-t-xl" style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", minHeight: "50px" }}>        
        {$msg}    
        {$btnGroup}        
      </div>      
    )
  }
}

Footer.propTypes = {
  attemp: React.PropTypes.object.isRequired,
  onClick: React.PropTypes.func.isRequired,
  text: React.PropTypes.string.isRequired
};

export default Footer;
export {
  Warning
}