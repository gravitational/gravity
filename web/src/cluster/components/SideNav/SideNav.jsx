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

import React from "react";
import cfg from "app/config";
import styled from "styled-components";
import { FluxContext } from "app/components/nuclear";
import { withState } from "shared/hooks";
import { getters as navGetters } from "app/cluster/flux/nav";
import { getters as clusterGetters } from "app/flux/cluster";
import { Image, SideNav, SideNavItem } from "shared/components";
import SideNavItemIcon from "shared/components/SideNav/SideNavItemIcon";
import { NavLink, Link } from "react-router-dom";
import gravityIconSvg from 'shared/assets/images/gravitational-logo.svg';

export function ClusterSideNav({ logoSrc, navItems, version }) {
  const $items = navItems.map((item, index) => (
    <SideNavItem key={index} as={NavLink} exact={item.exact} to={item.to}>
      <SideNavItemIcon as={item.Icon} />
      {item.title}
    </SideNavItem>
  ));

  const $marketingBanner = !cfg.isEnterprise && (
    <StyledLogoItem
      bg="primary.light"
      mt="auto"
      borderTop="1px solid" borderLeft="none" borderColor="primary.dark" minHeight="40px"
      py="2" pl="5"
      as="a" href="https://gravitational.com/gravity/">
      <Image src={gravityIconSvg} maxHeight="18px" maxWidth="120px" ml="1" mr="2" />
      Try Gravity Enterprise
    </StyledLogoItem>
  )

  return (
    <SideNav>
      <StyledLogoItem py="2" pl="5" as={Link} to={cfg.getSiteRoute()}>
        <Image src={logoSrc} maxHeight="40px" maxWidth="120px" mr="3" />
        <span title={version}>{version}</span>
      </StyledLogoItem>
      <div
        style={{ display: "flex", flexDirection: "column", overflow: "auto" }}
      >
        {$items}
      </div>
      {$marketingBanner}
    </SideNav>
  );
}

const StyledLogoItem = styled(SideNavItem)`
  &:active {
    border-left-color: transparent;
    color: ${({ theme }) => theme.colors.text.primary};
  }

  > span {
    line-height: 1.4;
    text-overflow: ellipsis;
    overflow: hidden;
  }
`;

function mapState() {
  const reactor = React.useContext(FluxContext);
  const navStore = reactor.evaluate(navGetters.navStore);
  const clusterStore = reactor.evaluate(clusterGetters.clusterStore);
  return {
    navItems: navStore.sideNav,
    version: clusterStore.cluster.packageVersion,
    logoSrc: cfg.logo
  };
}

export default withState(mapState)(ClusterSideNav);