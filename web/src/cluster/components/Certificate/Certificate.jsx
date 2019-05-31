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
import { useFluxStore } from 'app/components/nuclear';
import * as Icons from 'shared/components/Icon';
import { withState } from 'shared/hooks';
import { getters } from 'app/cluster/flux/tlscert';
import { Box, Text, Flex } from 'shared/components';
import { ButtonWarning } from 'shared/components/Button';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';
import UpdateCertDialog from './UpdateCertDialog';

export function Certificate(props) {
  const { store, } = props;
  const [ isOpen, setIsOpen ] = React.useState(false);
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          HTTPS Certificate
        </FeatureHeaderTitle>
      </FeatureHeader>
      <StyledCardCert bg='primary.main'>
        <Flex bg='primary.light' pl={6} pr={4} py={4} alignItems="center">
          <Icons.License color="text.primary" fontSize={8} mr={2}/>
          <Text typography="subtitle1" bold color="primary.contrastText">
            {store.getToCn()}
          </Text>
          <ButtonWarning size="small" ml="auto" onClick={() => setIsOpen(true)}>
            Replace
          </ButtonWarning>
        </Flex>
        <Box bg='primary.main' p={6} color="primary.contrastText">
          <Text typography="body1" bold mb={2}>
            Issued To
          </Text>
          <CertAttr name="Common Name (CN)" value={store.getToCn()} />
          <CertAttr name="Organization (O)" value={store.getToOrg()} />
          <CertAttr name="Organization Unit (OU)" value={store.getToOrgUnit()} />
          <Text typography="body1" bold mt={4} mb={2}>
            Issued by
          </Text>
          <CertAttr name="Organization (O)" value={store.getByOrg()} />
          <CertAttr name="Organization Unit (OU)" value={store.getByOrgUnit()} />
          <Text typography="body1" bold mt={4} mb={2}>
            Validity Period
          </Text>
          <CertAttr name="Issued On" value={store.getStartDate()} />
          <CertAttr name="Expires On" value={store.getEndDate()} />
        </Box>
      </StyledCardCert>
      { isOpen && <UpdateCertDialog onClose={ () => setIsOpen(false) } /> }
    </FeatureBox>
  )
}

const StyledCardCert = styled(Box)`
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0, 0, 0, .24);
`

const CertAttr = ({ name, value }) => (
  <Box mb={2}>
    <span style={{ width: "180px", display: "inline-block" }}>{name}</span>
    <span>{value}</span>
  </Box>
)

export default withState(() => {
  const store = useFluxStore(getters.store);
  return {
    store
  }
})(Certificate);