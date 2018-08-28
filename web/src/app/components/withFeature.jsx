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
import { RestRespCodeEnum } from 'app/services/enums';
import Indicator from 'app/components/common/indicator';
import * as Messages from 'app/components/msgPage.jsx';
import Logger from 'app/lib/logger';

const logger = Logger.create('components/withFeature');

const withFeature = feature => component => {
  
  return class WithFeatureWrapper extends React.Component{
      
    constructor(props, context) {
      super(props, context)            
      this._unsubscribeFn = null;
    }
                    
    componentDidMount() {      
      try{
        this._unsubscribeFn = reactor.observe(feature.initAttemptGetter(), ()=>{        
          this.setState({})
        })

        reactor.batch(() => {
          feature.componentDidMount();
        })      
                
      }catch(err){
        logger.error('failed to initialize a feature', err);        
      }      
    }

    componentWillUnmount() {
      this._unsubscribeFn();
    }
             
    render() {      
      if (feature.isProcessing()) {
        return <Indicator delay="long" type="bounce" />;  
      }
      
      if (feature.isFailed()) {        
        const errorText = feature.getErrorText();
        if (feature.getErrorCode() === RestRespCodeEnum.FORBIDDEN) {
          return <Messages.Failed message={errorText}/>  
        }
        return <Messages.Failed message={errorText}/>
      }

      let props = this.props;
      return React.createElement(component, {
        ...props,
        feature
      });      
    }
  }
}

export default withFeature;