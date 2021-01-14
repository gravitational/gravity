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
import { download } from 'oss-app/services/downloader';
import { Flex, Box, Text, Card } from 'shared/components';
import * as Icons from 'shared/components/Icon';
import ActionButton, { MenuItem } from './ActionButton';
import VersionMenu from './VersionMenu';
import { AppKindEnum } from 'oss-app/services/applications';

export default function AppTile({ onInstall, apps, ...styles }) {
  const [version, changeVersion] = React.useState(apps[0].version);
  const app = apps.find(a => a.version === version);
  const allVersions = apps.map(a => a.version);
  const { name, kind, createdText, standaloneInstallerUrl } = app;

  const installBtnProps = {
    onClick: () => onInstall(app),
  };

  const LogoIcon = kind === AppKindEnum.APP ? Icons.Stars : Icons.Kubernetes;

  return (
    <Card
      minHeight="300px"
      px="3"
      py="4"
      width="300px"
      as={Flex}
      flexDirection="column"
      {...styles}
    >
      <VersionMenu mb="1" mt={-1} value={version} options={allVersions} onChange={changeVersion} />
      <Flex flex="1" mb="4" py="2" justifyContent="center" alignItems="center" flexDirection="column">
        <Box textAlign="center" mb="3" width="100%">
          <Text typography="h3">
            {name}
          </Text>
          <Text typography="body2" color="text.primary">
            CREATED: {createdText}
          </Text>
        </Box>
        <LogoIcon color="text.primary" fontSize="100px" />
      </Flex>
      <ActionButton
        alignSelf="center"
        mb="2"
        width="210px"
        btnText="INSTALL IMAGE"
        buttonProps={installBtnProps}
      >
        <MenuItem {...installBtnProps}>INSTALL IMAGE</MenuItem>
        <MenuItem onClick={() => download(standaloneInstallerUrl)}>DOWNLOAD</MenuItem>
      </ActionButton>
    </Card>
  );
}
