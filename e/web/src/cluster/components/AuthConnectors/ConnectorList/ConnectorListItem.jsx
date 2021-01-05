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
import { Text, Flex, ButtonPrimary } from 'shared/components';
import ActionMenu, { MenuItem } from 'oss-app/cluster/components/components/ActionMenu';
import getSsoIcon from './../getSsoIcon';

export default function ConnectorListItem({ name, kind, id, onEdit, onDelete, ...rest }) {
  const onClickEdit = () => onEdit(id);
  const onClickDelete = () => onDelete(id);
  const { desc, SsoIcon } = getSsoIcon(kind);

  return (
    <StyledConnectorListItem
      width="300px"
      height="300px"
      borderRadius="3"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      bg="primary.light"
      px="5"
      pt="4"
      pb="5"
      {...rest}
    >
      <Flex width="100%" justifyContent="center">
        <Text typography="h3" caps bold>
          {name}
        </Text>
        <ActionMenu buttonIconProps={menuActionProps}>
          <MenuItem onClick={onClickDelete}>Delete...</MenuItem>
        </ActionMenu>
      </Flex>
      <Flex flex="1" mb="3" alignItems="center" justifyContent="center" flexDirection="column">
        <SsoIcon height="100px" width="160px" fontSize="100px" my={2} />
        <Text typography="body2" color="text.primary">
          {desc}
        </Text>
      </Flex>
      <ButtonPrimary mt="auto" size="medium" block onClick={onClickEdit}>
        EDIT CONNECTOR
      </ButtonPrimary>
    </StyledConnectorListItem>
  );
}

const StyledConnectorListItem = styled(Flex)`
  position: relative;
  transition: all 0.3s;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.24);
  &:hover {
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.56);
  }
`;

const menuActionProps = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};