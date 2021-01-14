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

import React from 'react'
import { NavLink, Link } from 'react-router-dom';
import TopNavUserMenu from 'shared/components/TopNav/TopNavUserMenu'
import hubLogo from 'shared/assets/images/gravity-hub.svg';
import { Image, Flex, ButtonPrimary, TopNav, TopNavItem } from 'shared/components';
import { MenuItem } from 'shared/components/Menu/';
import cfg from 'e-app/config';

export default class HubTopNav extends React.Component {

  state = {
    open: false,
  };

  onShowMenu = () => {
    this.setState({ open: true });
  };

  onCloseMenu = () => {
    this.setState({ open: false });
  };

  onItemClick = () => {
    this.onClose();
  }

  onLogout = () => {
    this.onCloseMenu();
    this.props.onLogout();
  }

  render() {
    const { userName, items, pl } = this.props;
    const { open } = this.state;

    const $items = items.map((item, index) => (
      <TopNavItem px="5" as={NavLink} exact={item.exact} key={index} to={item.to}>
        {item.title}
      </TopNavItem>
    ))

    return (
      <TopNav height="72px" pl={pl} style={{ "zIndex": "1", "boxShadow": "0 4px 16px rgba(0,0,0,.24)" }}>
        <TopNavItem pr="5" as={Link} to={cfg.routes.defaultEntry}>
          <Image src={hubLogo}  ml="3" mr="4" maxHeight="40px" maxWidth="160px" />
        </TopNavItem>
        {$items}
        <Flex ml="auto" height="100%">
          <TopNavUserMenu
            menuListCss={menuListCss}
            open={open}
            onShow={this.onShowMenu}
            onClose={this.onCloseMenu}
            user={userName}>
            <MenuItem>
              <ButtonPrimary my={3} block onClick={this.onLogout}>
                Sign Out
              </ButtonPrimary>
            </MenuItem>
          </TopNavUserMenu>
        </Flex>
      </TopNav>
    )
  }
}

const menuListCss = () => `
  width: 250px;
`