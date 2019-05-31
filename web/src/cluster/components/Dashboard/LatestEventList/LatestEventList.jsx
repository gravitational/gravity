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
import { Flex, Box, Text } from 'shared/components';
import { connect } from 'app/components/nuclear';
import EventList from './../../Audit/EventList';
import { getters } from 'app/cluster/flux/events';
import AjaxPoller from 'app/components/dataProviders'
import { fetchLatest } from 'app/cluster/flux/events/actions';

const POLL_INTERVAL = 5000; // every 5 sec

function LatestEventList(props) {
  const { store, onRefresh, ...styles } = props;
  const events = store.getEvents();
  return (
    <Box {...styles}>
      <Flex  bg="primary.light" p="3" alignItems="center" justifyContent="space-between">
        <Text typography="h4">
          Audit Logs
        </Text>
        <Text typography="body2" color="text.primary">
          VIEW ALL
        </Text>
      </Flex>
      <EventList events={events} limit="4"/>
      <AjaxPoller time={POLL_INTERVAL} onFetch={onRefresh} />
    </Box>
  );
}

const subToStore = () => {
  return {
    store: getters.store,
  }
}

function mapActions(){
  return {
    onRefresh: fetchLatest,
  }
}

export default connect(subToStore, mapActions)(LatestEventList);