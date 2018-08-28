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
import './box.scss'
import Layout from 'app/components/common/layout';
import Separator from './../separator';

let Box = React.createClass({

  renderBoxHeader() {
    let { title } = this.props;

    if (!title) {
      return null;
    }

    return (
      <BoxHeader>        
        <h3>{title}</h3>                
      </BoxHeader>
    )
  },

  render() {
    let { className, style} = this.props;
    let containerClass = 'grv-box-container';

    if (className) {
      containerClass = `${containerClass} ${className}`;
    }

    let $header = this.renderBoxHeader();
    let boxStyle = { ...style };

    return (
      <div style={boxStyle} className={containerClass}>
        {$header}
        <div className="grv-box-content">
          {this.props.children}
        </div>
      </div>
    );
  }
});

let BoxHeader = React.createClass({
  render() {
    return (
      <div className="grv-box-header">
        <Layout.Flex className="" dir="column" align="center">
          {this.props.children}
        </Layout.Flex>
        <Separator />
      </div>
    )
  }
});

Box.Header = BoxHeader

export default Box;
