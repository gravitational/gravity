/*
Copyright 2018 Gravitational, Inc.

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
import classnames from 'classnames';

const SizeEnum = {
  SM: 'sm',
  XS: 'xs',
  DEFAULT: 'default'
}

const styles = {
  size: {
    [SizeEnum.DEFAULT]: {
      minWidth: '60px'
    },
    [SizeEnum.SM]: {
      minWidth: '60px'
    }
  }
}

const getStyle = size => {  
  return {
    ...styles.size[size]
  }
}

const hideContentStyle = {
  visibility: 'collapse',
  height: '0px'
}

const onClick = (e, props) => {
  e.preventDefault();
  let {isProcessing, isDisabled} = props;
  if (isProcessing || isDisabled) {
    return;
  }

  props.onClick();
};

const Button = props => {
  let {
    size,
    title,
    isProcessing = false,
    isBlock = false,
    className = '',
    children,
    isDisabled = false} = props;

  let containerClass = classnames('btn ' + className, {
    'disabled': isDisabled,
    'btn-block': isBlock,
    'btn-sm': size === SizeEnum.SM,
    'btn-xs': size === SizeEnum.XS,
  });
        
  let containerStyle = getStyle(size);
  let contentStyle = isProcessing ? hideContentStyle : {};
  let $icon = isProcessing ? <i className="fa fa-cog fa-spin fa-lg"></i> : null;
  return (
    <button
      style={containerStyle}
      title={title}
      className={containerClass}
      onClick={e => onClick(e, props)}>
      {$icon}
      <div style={contentStyle}>
        {children}
      </div>
    </button>
  );
}

export default Button;


