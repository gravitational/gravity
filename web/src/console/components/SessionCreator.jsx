/*
Copyright 2019 Gravitational, Inc.

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
import styled from 'styled-components';
import cfg from 'app/config';
import { Redirect, Route, Switch } from 'app/components/Router';
import Terminal from './Terminal';
import Player from './Player';
import { colors } from './colors';
import { useAttempt } from 'shared/hooks';
import * as actions from './../flux/terminal/actions';
import { Indicator, Box } from 'shared/components';
import * as Alerts from 'shared/components/Alert';

export default function Console() {
  return (
    <StyledConsole>
      <Switch>
        <Route path={cfg.routes.consoleSession} component={Terminal} />
        <Route path={cfg.routes.consoleInitSession} component={SessionCreator} />
        <Route path={cfg.routes.consoleInitPodSession} component={SessionCreator} />
        <Route path={cfg.routes.consoleSessionPlayer} component={Player} />
      </Switch>
    </StyledConsole>
  )
}

function SessionCreator({ match }){
  const { siteId, pod, namespace, container, serverId, login } = match.params;
  const [ sid, setSid ] = React.useState();
  const [ attempt, { error } ] = useAttempt({
    isProcessing: true
  });

  React.useEffect(() => {
    actions.createSession({ siteId, serverId, login, pod, namespace, container })
      .then(sessionId => {
        setSid(sessionId)
      })
      .fail(err => {
        error(err)
      })
  }, [ siteId ]);

  // after obtaining the session id, redirect to a terminal
  if(sid){
    const route = cfg.getConsoleSessionRoute({ siteId, sid });
    return <Redirect to={route}/>
  }

  const { isProcessing, isFailed } = attempt;

  if(isProcessing){
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    )
  }

  if(isFailed){
    return (
      <Alerts.Danger m={10}>
        Connection error: {status.errorText}
      </Alerts.Danger>
    )
  }

  return null;
}

const StyledConsole = styled.div`
  background-color: ${colors.bgTerminal};
  bottom: 0;
  left: 0;
  position: absolute;
  right: 0;
  top: 0;
`;