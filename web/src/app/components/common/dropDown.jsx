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
import ReactSelect from 'react-select';
import $ from 'jQuery';
import {isObject} from 'lodash';
import classnames from 'classnames';

const DropDown = React.createClass({

  onClick(event){
    event.preventDefault();
    const {options} = this.props;
    const index = $(event.target).parent().index();
    const option = options[index];
    const value = isObject(option) ? option.value : option;

    this.props.onChange(value);
  },

  renderOption(option, index){
    const displayValue = isObject(option) ? option.label : option;
    return (
      <li key={index}>
        <a href="#">{displayValue}</a>
      </li>
    )
  },

  getDisplayValue(value){
    const {options=[]} = this.props;
    for(let i = 0; i < options.length; i++){
      let op = options[i];
      if(isObject(op) && op.value === value){
        return op.label;
      }

      if(op === value){
        return value;
      }
    }

    return null;
  },

  render(){
    const {options, value, classRules, name, right=false, size='default'} = this.props;
    const $options = options.map(this.renderOption);
    const hiddenValue = value;
    const displayValue = this.getDisplayValue(value) || 'Select...';
    
    const valueClass = classnames('grv-dropdown-value', { 'text-muted': !hiddenValue })
    const menuClass = classnames('dropdown-menu', { 'pull-right': right });
    const btnClass = classnames('btn btn-default full-width dropdown-toggle', {
      'btn-sm': size === 'sm'
    })
    
    return (
      <div className="grv-dropdown">
        <div className = "dropdown" >
          <div className={btnClass} type="button" id="dropdownMenu1" data-toggle="dropdown" aria-haspopup="true" aria-expanded="true">
            <div className={valueClass}>
              <span style={{textOverflow: "ellipsis", overflow: "hidden"}}>{displayValue}</span>
              <span className="caret m-l-sm"></span>
            </div>
          </div>
          {
            options.length > 0 &&
              <ul onClick={this.onClick} className={menuClass}>
                {$options}
              </ul>               
          }
        </div>
        <input className={classRules} value={hiddenValue} type="hidden" ref="input" name={name}/>
      </div>
    )
  }
});

const Select = React.createClass({

  onChange(value){
    $(this.refs.input).val(value);
    if(this.props.onChange){
      this.props.onChange(value);
    }
  },

  componentDidMount(){
    this.ensureValidationPlaceholder();
  },

  ensureValidationPlaceholder(){
    const { value } = this.props;
    $(this.refs.input).val(value);
  },

  componentDidUpdate(){
    this.ensureValidationPlaceholder();
  },

  render(){
    let props = this.props;
    let {classRules='', name, options} = props;

    // if just an array of strings, convert it to { value, label } format
    if(Array.isArray(options) && !isObject(options[0])){
      options = options.map( item => ({ value: item, label: item}));
    }

    let childProps = {
      searchable: false,
      clearable: false,
      ...props,
      options,
       onChange: this.onChange
    }

    return (
      <div>
        <ReactSelect {...childProps} ref="container"/>
        <input className={classRules} type="hidden" ref="input" name={name} />
    </div>
    )
  }
});

export {
  Select,
  DropDown
}
