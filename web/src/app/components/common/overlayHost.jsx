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

export function makeOverlayHost(Component) {

  class OverlayHost extends React.Component {
    getChildContext() {
      let self = this;
      return {
        getOverlayHost() {
          return self;
        }
      };
    }

    render() {
      let style = {
        ...this.props.style,
        position: "relative"
      }

      let props = this.props;


      return <Component {...props} style={style} />
    }
  }

  OverlayHost.childContextTypes = {
    getOverlayHost: React.PropTypes.func
  };

  return OverlayHost;
}

export default class OverlayHost extends React.Component {
  getChildContext() {
    let self = this;
    return {
      getOverlayHost() {
        return self;
      }
    };
  }

  render() {
    let child = React.Children.only(this.props.children);
    let style = {
      ...child.props.style,
      position: "relative"
    }

    return React.cloneElement(child, { style });
  }
}

OverlayHost.childContextTypes = {
  getOverlayHost: React.PropTypes.func
};