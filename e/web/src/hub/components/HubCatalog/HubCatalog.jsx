/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { values } from 'lodash';
import { useFluxStore } from 'oss-app/components/nuclear';
import CardEmpty from 'oss-app/components/CardEmpty';
import { withState } from 'shared/hooks';
import { Flex } from 'shared/components';
import { getters } from 'e-app/hub/flux/catalog';
import AppTileList from './AppTileList';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from '../components/Layout';

export function HubCatalog({ apps }){
  apps = apps || [];
  const isEmpty = apps.length === 0;
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          Catalog
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Flex flexWrap="wrap">
        { isEmpty && <CardEmpty title="There are no images"/> }
        { !isEmpty && <AppTileList apps={apps} /> }
      </Flex>
    </FeatureBox>
  )
}

function mapState(){
  const catalogStore = useFluxStore(getters.catalogStore);
  const apps = values(catalogStore.apps);
  return {
    apps,
  }
}

export default withState(mapState)(HubCatalog);