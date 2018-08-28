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
import { findIndex, debounce} from 'lodash';

const WINDOW_RESIZE_DEBOUNCE_DELAY = 100;

var stepIndicactor = React.createClass({

  propTypes: {
   options: React.PropTypes.array.isRequired
  },

  getInitialState(){
    return {containerWidth: 100};
  },

  componentDidMount: function() {
    this.debouncedResize = debounce(
      this._handleResize,
      WINDOW_RESIZE_DEBOUNCE_DELAY
    );

    window.addEventListener('resize', this.debouncedResize);
    this._handleResize();
  },

  componentWillUnmount: function() {
    this.debouncedResize.cancel();
    window.removeEventListener('resize', this.debouncedResize);
  },

  _handleResize(){
    var containerWidth = this.refs.container.clientWidth;
    this.setState({containerWidth});
  },

  renderProgressLine(valueIndex, valueWidth){
    let { containerWidth } = this.state;
    let { options } = this.props;
    let progressLineActiveWidth = (100 / (options.length - 1)) * valueIndex;
    let width = containerWidth - valueWidth;

    let style = {
      width: `${width}px`,
      marginLeft: `${valueWidth / 2}px`
    }

    let completedLineStyle = {
      width: `${progressLineActiveWidth}%`
    }

    return (
      <div
        className="grv-installer-step-indicator-line"
        style={style}>
        <div
          className="progress-bar progress-bar-info"
          role="progressbar"
          aria-valuenow="20"
          aria-valuemin="0"
          aria-valuemax="100"
          style={completedLineStyle}>
        </div>
      </div>
    )
  },

  render() {
    let {value=0, options} = this.props;
    let {containerWidth} = this.state;

    let valueIndex = findIndex(options, o => o.value === value);
    let valueWidth = (containerWidth) / (options.length);

    let $bubles = [];
    let $bublesDescription = [];
    let $progressLine = this.renderProgressLine(valueIndex, valueWidth);

    options.forEach((item, index) => {
      let itemClass = classnames('grv-item ', {
        'grv-active': index <= valueIndex
      });

      let { title } = options[index];
      let offset = valueWidth * index;

      let style = {
        width: `${valueWidth}px`,
        position: 'absolute',
        left: `${offset}px`,
        textAlign: 'center'
      }

      $bublesDescription.push(<div key={"d" + index} style={style} className="text-uppercase">{title}</div>);
      $bubles.push(<div key={index} style={style}><div className={itemClass}/> </div>);
    });

    return (
      <div ref="container" className="grv-installer-step-indicator">
        { $progressLine }
        <div className="grv-installer-step-indicator-steps">
          {$bubles}
        </div>
        <div className="grv-installer-step-indicator-steps-description">
          {$bublesDescription}
        </div>
      </div>
    );
  }
});

export default stepIndicactor
