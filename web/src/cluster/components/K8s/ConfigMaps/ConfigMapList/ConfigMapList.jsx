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
import { displayDateTime } from 'app/lib/dateUtils';
import CardEmpty from 'app/components/CardEmpty';
import styled from 'styled-components';
import { Text, Box, Flex, ButtonPrimary } from 'shared/components';
import * as Icons from 'shared/components/Icon';

export default function ConfigMapList({items, namespace, onEdit}){
  if(items.length === 0){
    return (
      <CardEmpty title="No Config Maps Found">
        <Text>
          There are no config maps for the "<Text as="span" bold>{namespace}</Text>" namespace
        </Text>
      </CardEmpty>
    )
  }

  return items.map(item => {
    const { name, created } = item;
    return (
      <ConfigMapListItem
        key={name}
        name={name}
        created={created}
        onClick={() => onEdit(name)}
      />
    )
  })
}

function ConfigMapListItem({onClick, created, name}) {
  const displayCreated = displayDateTime(created);
  return (
    <StyledConfigMapListItem bg="primary.light" px="4" py="4" mb={4} mr={4}>
      <Flex width="100%" textAlign="center" flexDirection="column" justifyContent="center">
        <Text typography="h6" mb="1">{name}</Text>
        <Text typography="body2" color="text.primary">EDITED: {displayCreated}</Text>
      </Flex>
      <Flex alignItems="center" justifyContent="center" flexDirection="column">
        <Icons.FileCode fontSize="50px" my={4} />
        <ButtonPrimary mt={3} size="medium" onClick={onClick} block={true}>
          EDIT CONFIG MAP
        </ButtonPrimary>
      </Flex>
    </StyledConfigMapListItem>
  );
}

const StyledConfigMapListItem = styled(Box)`
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, .24);
  width: 260px;
  &:hover {
    box-shadow: 0 24px 64px rgba(0, 0, 0, .56);
  }
`