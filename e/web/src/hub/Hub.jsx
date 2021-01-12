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
import styled from 'styled-components';
import { Redirect, Switch, Route } from 'oss-app/components/Router';
import session from 'oss-app/services/session';
import userGetters from 'oss-app/flux/user/getters';
import { useFluxStore } from 'oss-app/components/nuclear';
import { getters as navGetters } from 'e-app/hub/flux/nav';
import { Indicator  } from 'shared/components';
import { withState, useAttempt  } from 'shared/hooks';
import { Failed } from 'shared/components/CardError';
import { initHub } from './flux/actions';
import FeatureHubLicenses from './features/featureHubLicenses';
import FeatureHubClusters from './features/featureHubClusters';
import FeatureHubCatalog from './features/featureHubCatalog';
import FeatureHubSettings from './features/featureHubSettings';
import FeatureHubAccess from './features/featureHubAccess';
import HubTopNav from './components/HubTopNav';
import * as Layout from './components/components/Layout';
import 'oss-app/flux';
import './flux';
import cfg from 'e-app/config';

export function Hub({ features, attempt, userName, onLogout, navItems }) {
  const { isFailed, isSuccess, message } = attempt;

  if(isFailed){
    return <Failed message={message} />;
  }

  if(!isSuccess){
    return (
      <StyledIndicator>
        <Indicator />
      </StyledIndicator>
    )
  }

  const allowedFeatures = features.filter( f => !f.isDisabled() );
  const $features = allowedFeatures.map((item, index) => {
    const { path, title, exact, component } = item.getRoute();
    return (
      <Route
        title={title}
        key={index}
        path={path}
        exact={exact}
        component={component}
      />
    )
  })

  // handle index route
  const indexTab = navItems.length > 0 ? navItems[0].to : null;

  return (
    <StyledLayout>
      <HubTopNav
        userName={userName}
        onLogout={onLogout}
        items={navItems}
      />
      <Switch>
        { indexTab && <Redirect exact from={cfg.routes.defaultEntry} to={indexTab}/> }
        {$features}
      </Switch>
    </StyledLayout>
  );
}

const StyledLayout = styled.div`
  flex-direction: column;
  position: absolute;
  width: 100%;
  height: 100%;
  display: flex;
  overflow: hidden;
`

const StyledIndicator = styled(Layout.AppVerticalSplit)`
  align-items: center;
  justify-content: center;
`

export default withState(() => {
  // global stores
  const userStore = useFluxStore(userGetters.user);
  const navStore = useFluxStore(navGetters.navStore);

  // local state
  const [ attempt, attemptActions ] = useAttempt();
  const [ features ] = React.useState(() => {
    return [
      new FeatureHubClusters(),
      new FeatureHubCatalog(),
      new FeatureHubLicenses(),
      new FeatureHubAccess(),
      new FeatureHubSettings(),
    ]
  });

  React.useEffect(() => {
    // initialize hub stores
    attemptActions.do(() => initHub(features));
  }, []);

  const userName = userStore ? userStore.userId : '';

  return {
    features,
    attempt,
    userName,
    navItems: navStore.topNav,
    onLogout: () => session.logout(),
  }
})(Hub);