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

const DEFAULT_DELAY = "short"; 

const DelayValueMap = {
  "none": 0,
  "short": 400, // 0.4s;
  "long": 600,  // 0.6s;
}

class Indicator extends React.Component {

  constructor(props) {
    super(props);    
    this._timer = null;
    this._delay = props.delay || DEFAULT_DELAY;
    this.state = {
      canDisplay: false
    }
  }

  componentDidMount() {        
    let timeoutValue = DelayValueMap[this._delay];
    this._timer = setTimeout(() => {
      this.setState({
        canDisplay: true
      })
    }, timeoutValue);
  }
  
  componentWillUnmount() {
    clearTimeout(this._timer);
  }
  
  render() {    
    let { type = 'bounce' } = this.props;
    
    if (!this.state.canDisplay) {
      return null;
    }

    if (type === 'bounce') {
      return <ThreeBounce />
    }

    if (type === 'circle') {
      return <Circle />
    }
  }
}

const ThreeBounce = () => (
  <div className="grv-spinner sk-spinner sk-spinner-three-bounce">
    <div className="sk-bounce1"/>
    <div className="sk-bounce2"/>
    <div className="sk-bounce3"/>
  </div>
)
  
const Circle = () => (
  <div className="sk-spinner sk-spinner-circle">
    <div className="sk-circle1 sk-circle"/>
    <div className="sk-circle2 sk-circle"/>
    <div className="sk-circle3 sk-circle"/>
    <div className="sk-circle4 sk-circle"/>
    <div className="sk-circle5 sk-circle"/>
    <div className="sk-circle6 sk-circle"/>
    <div className="sk-circle7 sk-circle"/>
    <div className="sk-circle8 sk-circle"/>
    <div className="sk-circle9 sk-circle"/>
    <div className="sk-circle10 sk-circle"/>
    <div className="sk-circle11 sk-circle"/>
    <div className="sk-circle12 sk-circle"/>
  </div>
)
  
export default Indicator;