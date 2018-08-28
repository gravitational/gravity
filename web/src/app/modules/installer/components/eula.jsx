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
import ReactDOM from 'react-dom';
import { AppLogo } from './items';
import Button from 'app/components/common/button';

const Eula = React.createClass({

  propTypes: {
   content: React.PropTypes.string.isRequired,
   eulaHeaderText: React.PropTypes.string.isRequired,
   eulaAgreeText: React.PropTypes.string.isRequired,
   onAccept: React.PropTypes.func.isRequired
  },
  
  getInitialState(){
    return {
      accepted: false      
    }
  },
    
  componentDidMount() {     
    let logoContainer = ReactDOM.findDOMNode(this.refs.logoElement);
    let headerTextContainer = ReactDOM.findDOMNode(this.refs.headerTextElement);
    // give some time to display an image
    setTimeout(()=> {
      let logoWidth = logoContainer.offsetWidth;
      // since logo width differs, make sure that header text is centered.
      headerTextContainer.style.width = `calc(100% - ${logoWidth}px`;     
    }, 10);
  },
  
  onChange(){
    let { accepted } = this.state;
    this.setState({
      accepted: !accepted
    })
  },
  
  render() {    
    let {
      content,
      appName,
      onAccept,
      eulaContentLabelText,
      eulaHeaderText,
      logoUri,
      eulaAgreeText
    } = this.props;

    let { accepted } = this.state;

    let headerText = eulaHeaderText.replace('{0}', appName);

    return (
      <div className="grv-installer" >
        <div className="grv-installer-eula container">                        
          <div>
            <div className="grv-installer-header">
              <div ref="logoElement" style={{ flex: "0", alignSelf: "center" }}>
                <AppLogo logoUri={logoUri} />                
              </div>                         
              <div style={{ flex: "1", alignSelf: "center" }} className="text-center">
                <h1 ref="headerTextElement" className="no-margins">{headerText}</h1>
              </div>
            </div>
          </div>
          <div className="m-t">
            <h2>{eulaContentLabelText}</h2>            
          </div>
          <div className="m-t grv-installer-eula-content">
            {content}
          </div>          
          <div className="m-t-lg m-b-lg">
            <div className="grv-installer-eula-footer">
              <div className="grv-installer-eula-footer-agree m-b-sm">
                <input 
                  type="checkbox" 
                  id="grv-agree-chk-box" 
                  defaultChecked={accepted}
                  onChange={this.onChange}
                  />
                <label className="m-l-xs" htmlFor="grv-agree-chk-box">
                  {eulaAgreeText}
                </label>              
              </div>
              <Button onClick={onAccept} isDisabled={!accepted} className="btn-primary btn-block" >
                <span>Accept</span>
              </Button>
            </div>            
          </div>        
        </div>                            
      </div>      
    );
  }
});

export default Eula;
