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

import connect from 'app/lib/connect'
import { getters as tlsGetters } from '../../flux/tls';
import settingsGetters from '../../flux/getters';
import { Certificate } from './certificate';

const mapFluxToProps = () => {
  return {      
    store: tlsGetters.store,  
    siteId: settingsGetters.siteId
  }
}

export default connect(mapFluxToProps)(Certificate);