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
    color: ${props.theme.colors.grey[400]};
    font-size: 12px;
    border-bottom: 1px solid ${props.theme.colors.subtle };
    line-height: 32px;
    margin: 0 8px;
    padding: 0 8px;

    &:hover {
      color: ${props.theme.colors.link};
    }

    &:last-child {
      border-bottom: none;
    }
  }
`

const StyledCliIcon = styled.div`
  align-items: center;
  background: ${props => props.theme.colors.bgTerminal };
  display: flex;
  flex: 1;
  opacity: .87;
  padding: 0 0 0 8px;
  transition: all .3s;
`

const ButtonIcon = styled.div`
  align-items: center;
  background: ${props => props.theme.colors.bgTerminal };
  display: flex;
  flex: 1;
  opacity: .24;
  padding: 0 0 0 8px;
  transition: all .3s;
`

const StyledSession = styled.div`
  align-items: center;
  background: ${props => props.theme.colors.bgTerminal };
    border: 1px solid  ${props => props.theme.colors.bgTerminal};
  border-radius: 2px;
  box-sizing: border-box;
  box-shadow: 0 0 2px rgba(0, 0, 0, .12),  0 2px 2px rgba(0, 0, 0, .24);
  color: ${props => props.theme.colors.primary};
  cursor: pointer;
  display: flex;
  height: 24px;
  position: relative;
  width: 56px;
  transition: all .3s;

  &:hover {
    border: 1px solid  ${props => props.theme.colors.success};
    box-shadow:  0 4px 16px rgba(0, 0, 0, .24);

    ${StyledCliIcon} {
      opacity: 1;
    }
    ${ButtonIcon} {
      opacity: .56
    }
  }
`;


const Input = styled.input`
  background: ${props => props.theme.colors.subtle };
  border: 1px solid ${props => props.theme.colors.subtle };
  border-radius: 4px;
  box-sizing: border-box;
  color: #263238;
  padding: 0 8px;
  height: 32px;
  margin: 8px;
  outline: none;

  &:focus {
    background: ${props => props.theme.colors.light };
    border 1px solid ${props => props.theme.colors.link };
    box-shadow: inset 0 1px 3px rgba(0, 0, 0, .24);
  }

  ::-webkit-input-placeholder { /* Chrome/Opera/Safari */
  color: ${props => props.theme.colors.grey[100]};
  }
  ::-moz-placeholder { /* Firefox 19+ */
    color: ${props => props.theme.colors.grey[100]};
  }
  :-ms-input-placeholder { /* IE 10+ */
    color: ${props => props.theme.colors.grey[100]};
  }
  :-moz-placeholder { /* Firefox 18- */
    color: ${props => props.theme.colors.grey[100]};
  }
`

export default MenuLogin;
