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
import Profile from './../Elements/Profile';
import CmdText from 'app/components/CmdText';
import { Server } from './Server';
import { Box, LabelInput } from 'shared/components';

export default function ProfileOnprem(props){
  const {
    name,
    servers,
    count,
    description,
    instructions,
    requirementsText,
    onSetServerVars,
    onRemoveServerVars,
    mb
  } = props;

  const $servers = servers.map(server => (
    <Server
      mx={-4} px="4" pt="3"
      key={name+server.hostname}
      role={server.role}
      hostname={server.hostname}
      vars={server.vars}
      onSetVars={onSetServerVars}
      onRemoveVars={onRemoveServerVars}
    />
  ))

  return (
    <Profile
      count={count}
      requirementsText={requirementsText}
      description={description}
      mb={mb}
    >
      <LabelInput mt="3">
        Copy and paste the command below into terminal. Your server will automatically appear in the list
      </LabelInput>
      <CmdText cmd={instructions} />
      <Box mt="4">
        {$servers}
      </Box>
    </Profile>
  )
}
