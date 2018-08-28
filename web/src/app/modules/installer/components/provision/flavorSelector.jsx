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
import Slider from 'app/components/common/slider';

class FlavorSlider extends Slider{

  constructor(props){
    super(props);
  }

  setState(newState, cb){
    super.setState(newState, cb);
  }
}

var Value = ({offset, marginLeft}) => {

  let props = {
    className: 'grv-slider-value',
    style: {
      position: 'absolute',
      left: `${offset}px`,
      marginLeft: `${marginLeft}px`
    }
  }

  return ( <div {...props} /> );
}

var ValueDesc = ({offset, width, marginLeft, text}) => {
  let props = {
    className: 'grv-slider-value-desc',
    style: {
      width: `${width}px`,
      position: 'absolute',
      marginLeft:`${(width/-2) + marginLeft}px`,
      left: `${offset}px`,
      textAlign: 'center'
    }
  }

  return (
    <div {...props} >
      <span>{text}</span><br/>
    </div>
  )
}


var FlavorValueComponent = React.createClass({
  render(){
    let {options, handleSize, upperBound, sliderLength} = this.props;
    let $vals = [];
    let $descriptions = [];
    let count = options.length - 1;

    let widthWithHandle = upperBound / count;
    let widthWithoutHandle = sliderLength / count;
    let marginLeft = handleSize / 2;

    for(var i = 0; i < options.length; i++){
      let offset = widthWithHandle * i;
      let { label } = options[i];

      let valueProps = {
        key: 'value_' + i,
        offset,
        marginLeft
      }

      let descProps = {
        ...valueProps,
        key: 'desc_' + i,
        width: widthWithoutHandle,
        text: label
      }

      $vals.push(<Value {...valueProps}/>);
      $descriptions.push(<ValueDesc {...descProps}/>);
    }

    return (
      <div>
        <div>{$vals}</div>
        <div className="grv-installer-provision-flavors-range" style={{position: 'absolute', width: '100%'}}>{$descriptions}</div>
      </div>
    );
  }
})

export const FlavorSelector = React.createClass({
  propTypes: {
    current: React.PropTypes.number.isRequired,
    options: React.PropTypes.array
  },

  onChange(value){
    this.props.onChange(value-1);
  },

  render() {
    let {current, options} = this.props;
    let total = options.length;

    if(total < 2){
      return null;
    }

    return (
      <div className="grv-installer-provision-flavors p-w-sm">
        <FlavorSlider
          options={options}
          valueComponent={<FlavorValueComponent options={options}/>}
          min={1}
          max={total}
          value={current+1}
          onChange={this.onChange}
          onCanChange={this.onCanChange}
          defaultValue={1}
          withBars={true}
          className="grv-slider"/>
      </div>
    );
  }
})