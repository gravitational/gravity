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
import $ from 'jQuery';
import moment from 'moment';
import { debounce } from 'lodash';
import 'assets/js/datepicker';

var DateRange = React.createClass({

  getDefaultProps() {
     return {
       startDate: Date.now(),
       onChange: () => {}
     };
   },

  componentWillReceiveProps({value}){
    let currentValue = this.getDate();
    if(value != currentValue && !moment(currentValue).isSame(value)){
      this.setDate(value);
    }
  },

  shouldComponentUpdate(){
    return false;
  },

  componentDidMount(){
    let { settings } = this.props;
    let picker = $(this.refs.rangePicker).datepicker({
      ...settings,
      keyboardNavigation: false,
      forceParse: false,
      autoclose: true
    });

    this.setDate(this.props.value);

    this.onChange = debounce(this.onChange, 1);
    picker.on('changeDate', this.onChange);
  },

  componentWillUnmount(){
    $(this.refs.dp).datepicker('destroy');
  },

  onChange(){
    let value = this.getDate()
    if(!moment(this.props.value).isSame(value)){
      this.props.onChange(value);
    }
  },

  getDate(){
    return $(this.refs.dpPicker).datepicker('getDate');
  },

  setDate(value){
    $(this.refs.dpPicker).datepicker('setDate', value);
  },

  render() {
    let { placeholder, name } = this.props;

    let props = {
      name,
      placeholder,
      ref: 'dpPicker',
      type: 'text',
      className: 'form-control',
      style: {
        textAlign: 'left'
      }
    };

    return (
      <div className="grv-datepicker input-group input-daterange" ref="rangePicker">
        <input {...props} />
      </div>
    );
  }
});

export default DateRange;