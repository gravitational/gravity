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
import opProgressGetters from 'app/flux/opProgress/getters';
import { SiteOpProgressProvider } from './../dataProviders';
import reactor from 'app/reactor';
import classnames from 'classnames';
import { If } from 'app/components/common/helpers';

const Progress =  React.createClass({

  mixins: [reactor.ReactMixin],

  propTypes: {
   opId: React.PropTypes.string.isRequired
  },

  getDataBindings() {
    return {
      opProgress: opProgressGetters.progressById(this.props.opId)
    }
  },
  
  renderProgress(opId) {            
    if(!this.state.opProgress){
      return <SiteOpProgressProvider opId={opId}/>;
    }

    let { isError, step, message } = this.state.opProgress;
    
    step = step + 1;
    
    let progress = (step) * 10;
    let progressClass = classnames('grv-site-history-progress-indicator', {
      '--error': isError
    });

    let iconClassName = classnames('fa m-r-xs', {
      'fa-cog fa-lg fa-spin': !isError,      
      'fa-exclamation-triangle': isError
    });
          
    return (
      <div className="grv-site-history-progress m-t-sm m-b-sm">        
        <div className={progressClass}>
          <div className="grv-site-history-progress-indicator-line">
            <div
              className="progress-bar progress-bar-info"
              role="progressbar"
              aria-valuenow="20"
              aria-valuemin="0"
              aria-valuemax="100"
              style={{"width": progress + "%"}}>
            </div>
          </div>                                
        </div>
        <small className="text-muted">          
          <i className={iconClassName} aria-hidden="true" />
          <strong className="m-r-sm">Step {step} out of 10</strong>
          <div style={{ paddingLeft: "20px" }}>{message}</div>
        </small>          
        <SiteOpProgressProvider opId={opId}/>
      </div>
      )    
  },

  render(){
    let { opId, isProcessing, isCompleted, isFailed, isInitiated } = this.props;        
    if (isProcessing || isInitiated) {
      return this.renderProgress(opId);
    }                    

    let containerClass = classnames('grv-site-history-status m-t-sm m-b-sm', {
      '--error': isFailed,
      '--completed': isCompleted
    })
              
    return (
      <div className={containerClass}>              
        <If isTrue={isProcessing}>
          <strong className='text-warning'>Processing...</strong>
        </If>
        <If isTrue={isCompleted}>
          <strong className='text-success'>Completed</strong>
        </If>
        <If isTrue={isInitiated}>
          <strong className='text-warning'>Initiated</strong>
        </If>
        <If isTrue={isFailed}>
          <strong className='text-danger'>Failed</strong>
        </If>        
      </div>
    )
  }
});

export default Progress;