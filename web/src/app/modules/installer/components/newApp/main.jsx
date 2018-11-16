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
import connect from 'app/lib/connect';
import AwsProvider from './awsProvider';
import { ProviderEnum } from 'app/services/enums';
import { ProviderOptions } from './items';
import getters from './../../flux/newApp/getters';
import * as actions from './../../flux/newApp/actions';
import { Separator} from './../items.jsx';
import Footer from  './../footer.jsx';
import DomainName from './domainName';
import AdvancedOptions from './advanced';

class NewApp extends React.Component {

  componentDidMount(){
    const { availableProviders=[] } = this.props.newApp;
    // pre-select provider if only 1 is available
    if(availableProviders.length === 1){
      setTimeout( () => this.setDefaultProvider(availableProviders[0]), 0);
    }
  }

  setDefaultProvider(providerName){
    actions.setProvider(providerName);
    // since provider component may change focus,
    // ensure that focus is set back on first installer input element
    const tabbables = document.querySelectorAll("input");
    if(tabbables.length > 0){
      tabbables[0].focus();
    }
  }

  onChangeProvider = providerName => {
    actions.setProvider(providerName);
  }

  onCreateSite = () => {
    actions.createSite();
  }

  renderProvider(provName){
    if(provName === ProviderEnum.AWS){
      return <AwsProvider/>;
    }

    return null;
  }

  renderAdvancedOptions(newApp){
    const { enableTags, subnets, tags, selectedProvider } = newApp;
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
  }

  render() {
    const { createSiteAttempt, newApp} = this.props;
    const { availableProviders, selectedProvider } = newApp;
    const $provider = this.renderProvider(selectedProvider);
    const $advancedOptions = this.renderAdvancedOptions(newApp);

    return (
      <div ref="container">
        <DomainName/>
        <Separator/>
        <ProviderOptions options={availableProviders} value={selectedProvider} onChange={this.onChangeProvider}/>
        <div className="m-t-m">
          {$provider}
          {$advancedOptions}
        </div>
        <Footer text="Continue" attemp={createSiteAttempt} onClick={this.onCreateSite}/>
      </div>
    );
  }
}

function mapStateToProps() {
  return {
    newApp: getters.newApp,
    createSiteAttempt: getters.createSiteAttempt
  }
}

export default connect(mapStateToProps)(NewApp);

