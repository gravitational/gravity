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

import styled from 'styled-components'
import { flex, height, maxWidth, minHeight, maxHeight, minWidth, alignSelf, justifySelf, space, width, color, textAlign } from 'shared/system';
import defaultTheme from './../../theme'


const borderRadius = props => {
  const { br, brbr, brbl, brtr, brtl } = props;
  let radius = {
    borderTopLeftRadius:     0,
    borderTopRightRadius:    0,
    borderBottomRightRadius: 0,
    borderBottomLeftRadius:  0
  }

  if(br) {
    radius.borderTopLeftRadius =      br + 'px';
    radius.borderTopRightRadius =     br + 'px';
    radius.borderBottomRightRadius =  br + 'px';
    radius.borderBottomLeftRadius =   br + 'px';
  }

  if(brbr) {
    radius.borderBottomRightRadius = brbr + 'px';
  }

  if(brbl) {
    radius.borderBottomLeftRadius = brbl + 'px';
  }

  if(brtl) {
    radius.borderTopLeftRadius = brtl + 'px';
  }

  if(brtr) {
    radius.borderTopRightRadius = brtr + 'px';
  }

  return radius;
}

const Box = styled.div`
  box-sizing: border-box;
  ${borderRadius} ${maxWidth} ${minWidth} ${space} ${height} ${minHeight} ${maxHeight} ${width} ${color} ${textAlign} ${flex} ${alignSelf} ${justifySelf}
`

Box.displayName = 'Box';
Box.defaultProps = {
  theme: defaultTheme
};

Box.propTypes = {
  ...space.propTypes,
  ...height.propTypes,
  ...width.propTypes,
  ...color.propTypes,
  ...textAlign.propTypes,
  ...flex.propTypes,
  ...alignSelf.propTypes,
  ...justifySelf.propTypes,

}

export default Box;