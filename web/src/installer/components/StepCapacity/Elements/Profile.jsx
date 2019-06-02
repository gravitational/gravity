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
import {Card, Flex, Text, LabelState } from 'shared/components';

export default function Profile({count, description, requirementsText, children, ...styles }){
  return (
    <Card as={Flex} bg="primary.light" px="4" py="3" flexDirection="column" {...styles}>
      <Flex alignItems="center">
        <LabelState shadow width="100px" mr="6" py="2" fontSize="2" style={{flexShrink: "0"}}>
          {labelText(count)}
        </LabelState>
        <Flex flexDirection="colum" flexWrap="wrap" alignItems="baseline">
          <Text typography="h3" mr="4">
            {description}
          </Text>
          <Text as="span" typography="h6">
            { `REQUIREMENTS - ${requirementsText}`}
          </Text>
        </Flex>
      </Flex>
      {children}
    </Card>
  )
}

function labelText(count){
  const nodes = count > 1 ? 'nodes' : 'node';
  return `${count} ${nodes}`
}