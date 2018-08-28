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
import AwsProvider from './awsProvider';
import OnPremProvider from './onPremProvider';
import { ProviderEnum } from 'app/services/enums';
import { ProviderOptions } from './items';
import getters from './../../flux/newApp/getters';
import * as actions from './../../flux/newApp/actions';
import { Separator} from './../items.jsx';
import Footer from  './../footer.jsx';
import DomainName from './domainName';
import AdvancedOptions from './advanced';

const NewApp = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      newApp: getters.newApp,
      createSiteAttemp: getters.createSiteAttemp
    }
  },

  componentDidMount(){
    let { availableProviders=[] } = this.state.newApp;
    // pre-select provider if only 1 is available
    if(availableProviders.length === 1){
      setTimeout( () => this.setDefaultProvider(availableProviders[0]), 0);
    }
  },

  setDefaultProvider(providerName){
    actions.setProvider(providerName);
    // since provider component may change focus,
    // ensure that focus is set back on first installer input element
    let tabbables = document.querySelectorAll("input");
    if(tabbables.length > 0){
      tabbables[0].focus();
    }
  },

  onChangeProvider(providerName){
    actions.setProvider(providerName);
  },

  onChangeDomainName(domainName){
    actions.setDomainName(domainName);
  },

  onCreateSite(){
    actions.createSite();
  },

  renderProvider(provName){
    if(provName === ProviderEnum.AWS){
      return <AwsProvider/>;
    }else if(provName === ProviderEnum.ONPREM){
      return <OnPremProvider/>;
    }

    return null;
  },

  renderAdvancedOptions(){
    let { enableTags, subnets, tags, selectedProvider } = this.state.newApp;
    if( !enableTags || !selectedProvider ){
      return null;
    }

    return (
      <div className="row">
        <div className="col-sm-12 m-t-xs">
          <AdvancedOptions
            provider={selectedProvider}
            tags={tags}
            subnets={subnets}            
            onChangeTags={actions.setAppTags} />
        </div>
      </div>
    )
  },

  render() {
    let {createSiteAttemp} = this.state;
    let {availableProviders, selectedProvider} = this.state.newApp;
    let $provider = this.renderProvider(selectedProvider);
    let $advancedOptions = this.renderAdvancedOptions();

    return (
      <div ref="container">
        <DomainName/>
        <Separator/>
        <ProviderOptions options={availableProviders} value={selectedProvider} onChange={this.onChangeProvider}/>
        <div className="m-t-m">
          {$provider}
          {$advancedOptions}
        </div>
        <Footer text="Continue" attemp={createSiteAttemp} onClick={this.onCreateSite}/>
      </div>
    );
  }
});

export default NewApp;
