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
import { Flex } from 'shared/components';
import Menu, { MenuItem} from 'shared/components/Menu';
import Icon, * as Icons from 'shared/components/Icon';

class MenuLogin extends React.Component {

  static displayName = 'MenuLogin';

  static defaultProps = {
    menuListCss: () => { },
  }

  constructor(props){
    super(props)
    this.state = {
      open: false,
      anchorEl: null,
    }
  }

  openTerminal(login){
    const { serverId } = this.props;
    const url = cfg.getConsoleInitSessionRoute({ login, serverId });
    window.open(url);//, "", "toolbar=yes,scrollbars=yes,resizable=yes");
  }

  onOpen = () => {
    this.setState({ open: true });
  };

  onItemClick = login => {
    this.onClose();
    this.openTerminal(login);
  }

  onClose = () => {
    this.setState({ open: false });
  }

  onKeyPress = e => {
    if (e.key === 'Enter' && e.target.value) {
      this.onClose();
      this.openTerminal(e.target.value);
    }
  }

  render() {
    const {
      logins,
      serverId,
      anchorOrigin,
      transformOrigin,
    } = this.props;

    const { open } = this.state;
    return (
      <React.Fragment>
        <StyledSession ref={e => this.anchorEl = e } onClick={this.onOpen}>
          <StyledCliIcon>
            <Icons.Cli/>
          </StyledCliIcon>
          <ButtonIcon>
            <Icons.CarrotDown />
          </ButtonIcon>
        </StyledSession>
        <Menu
          menuListCss={menuListCss}
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
        >
          <LoginItemList
            logins={logins}
            serverId={serverId}
            onKeyPress={this.onKeyPress}
            onClick={this.onItemClick}
          />
        </Menu>
      </React.Fragment>
    );
  }
}

export const LoginItemList = ({logins, onClick, onKeyPress}) => {
  logins = logins || [];
  const $menuItems = logins.map((login, key) => {
    return (
      <MenuItem key={key} as="a" onClick={ () => onClick(login)}>
        {login}
      </MenuItem>
  )});

  return (
    <React.Fragment>
      <Flex>
        <Input onKeyPress={onKeyPress} type="text" autoFocus placeholder="Enter login name..."/>
      </Flex>
      {$menuItems}
    </React.Fragment>
  )
}

const menuListCss = props => `
 ${MenuItem} {
   color: ${props.theme.colors.link};
   max-width: 200px;
   > * {
    overflow: hidden;
    text-overflow: ellipsis;
   }
 }
`

const StyledCliIcon = styled.div`
  align-items: center;
  background: ${props => props.theme.colors.bgTerminal };
  display: flex;
  flex: 1;
  opacity: .56;
  padding: 0 8px;
`

const ButtonIcon = styled.button`
  align-items: center;
  background: ${props => props.theme.colors.primary.main };
  border: none;
  cursor: pointer;
  display: flex;
  height: 100%;
  justify-content: center;
  outline: none;
  width: 30px;

  ${Icon}{
    display: inline-block;
    font-size: 12px;
    opacity: .56;
  }
`

const StyledSession = styled.div`
  align-items: center;
  background: ${props => props.theme.colors.bgTerminal };
  border-radius: 2px;
  box-shadow: 0 0 2px rgba(0, 0, 0, .12),  0 2px 2px rgba(0, 0, 0, .24);
  color: ${props => props.theme.colors.primary};
  cursor: pointer;
  display: flex;
  height: 24px;
  position: relative;
  width: 70px;
  &:hover, &:focus {
    ${ButtonIcon}{
      background: ${props => props.theme.colors.primary.light };
    }
  }
`;

const Input = styled.input`
  background: #CFD8DC;
  border: 1px solid #CFD8DC;
  border-radius: 2px;
  width: 100%;
  box-sizing: border-box;
  color: #263238;
  padding: 0 8px;
  height: 40px;
  margin: 8px;
  &:focus {
    background: ${props => props.theme.colors.light };
    border 1px solid ${props => props.theme.colors.link };
    box-shadow: inset 0 2px 4px rgba(0, 0, 0, .24);
  }
`

export default MenuLogin;
