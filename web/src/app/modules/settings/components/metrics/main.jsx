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
import reactor from  'app/reactor';
import $ from 'jQuery';
import Button from 'app/components/common/button';
import Layout from 'app/components/common/layout';

import Box from 'app/components/common/boxes/box';
import getters from './../../flux/metrics/getters';
import { saveRetentionValues } from './../../flux/metrics/actions';

const VALIDATION_MAX_DEF = 730; // hours (~1 month)
const VALIDATION_MAX_MED = 26; // weeks (~6 months)
const VALIDATION_MAX_LONG = 260; // weeks (~5 years)

const MIN_IN_WEEK = 60 * 24 * 7;

const fromHours2Ms = h => h * 60000 * 60;
const fromMin2Ms = min => min * 60000;
const fromWeek2Ms = weeks => fromMin2Ms(weeks * MIN_IN_WEEK);
const fromMsToWeeks = ms => (ms / 60000) / MIN_IN_WEEK;
const fromMsToNano = ms => ms * 1000000;
const fromMsToHours = ms => ms / ( 60000 * 60)
const fromNanoToMs = nano => nano / 1000000;

const valStyle = {
  fontSize: "16px"  
}

const textStyle = {
  minWidth: "350px",
  textAlign: "left"
}

const Value = ({descriptionText, ruleName, duration, units="weeks", hintText, onChange}) => {                
  return (
    <Layout.Flex dir="row" align="start" className="form-group">                  
      <div style={textStyle}>                  
        <div style={valStyle}>
          {descriptionText}                                 
        </div>             
        <span className="text-muted">* {hintText}</span>                                             
      </div>                                  
      <Layout.Flex dir="row" align="center" className="m-l">
        <div style={{position: "relative"}}>          
          <input        
            type="number"     
            autoComplete="off"
            style={{width: "100px", textAlign: "center"}}            
            min="1"                        
            className="form-control"            
            value={duration}
            name={ruleName}
            onChange={ e => onChange(e.target.value)} />
            <div style={{position:"absolute", left: "110px", top: "8px"}} >{units}</div>                      
        </div>                          
      </Layout.Flex>                                          
    </Layout.Flex>
  )
}

const RetentionValues = React.createClass({

  propTypes: {
    defVal: React.PropTypes.number.isRequired,
    medVal: React.PropTypes.number.isRequired,
    longVal: React.PropTypes.number.isRequired,
    onSave: React.PropTypes.func.isRequired,
    isProcessing: React.PropTypes.bool,
    isFailed: React.PropTypes.bool,    
  },
    
  getInitialState(props){
    props = props || this.props;
    let { defVal, medVal, longVal } = props
    return {
      isDirty: false,
      defVal: fromMsToHours(defVal),
      medVal: fromMsToWeeks(medVal),
      longVal: fromMsToWeeks(longVal)
    };
  },

  componentWillReceiveProps(nextProps){
    let { defVal, medVal, longVal} = this.props;
    if(nextProps.defVal !== defVal || 
      nextProps.medVal !== medVal ||
      nextProps.longVal !== longVal){
        let newState = this.getInitialState(nextProps)
        this.setState(newState);
      }
  },

  componentDidMount(){    
    let rules = {
      number: true,
      required: true,
      min: 1,                
    }
        
    $(this.refs.form).validate({
      rules:{
        'defVal':{
          ...rules,                    
          max: VALIDATION_MAX_DEF,
          
        },
        'medVal':{
          ...rules,                    
          max: VALIDATION_MAX_MED
        },
        'longVal':{
          ...rules,                    
          max:  VALIDATION_MAX_LONG
        }
      }            
    })
  },

  onChangeDuration(name, newDuration){    
    this.setState({
      isDirty: true,
      [name]: newDuration             
    })
  },
  
  onSave(){    
    if($(this.refs.form).valid()){
      let { defVal, medVal, longVal} = this.state;
      this.props.onSave(
        fromHours2Ms(defVal), 
        fromWeek2Ms(medVal), 
        fromWeek2Ms(longVal));
    }
  },
    
  render(){            
    let { isDirty } = this.state;
    let { isProcessing } = this.props;
    let { defVal, medVal, longVal } = this.state;

    let descTextClass = 'grv-settings-metrics-ret-color';

    let defValDesc = (
      <span>Keep <span className={descTextClass}>high resolution real time</span> metrics for</span>
    )

    let medValDesc = (
      <span>Keep <span className={descTextClass}>medium resolution</span> metrics for</span>
    )

    let longValDesc = (
      <span>Keep <span className={descTextClass}>long term</span> metrics for</span>
    )

    return (
      <Box title="Metrics Retention" className="grv-settings-metrics-ret">        
        <form ref="form">
          <Value 
            ruleName="defVal"
            onChange={newVal => this.onChangeDuration('defVal', newVal)}
            descriptionText={defValDesc}
            hintText="measured in 10 second intervals"
            units="hours"            
            duration={defVal}
          />
          <Value 
            ruleName="medVal"            
            onChange={newVal => this.onChangeDuration('medVal', newVal)}
            descriptionText={medValDesc}
            hintText="measured in 5 minute intervals"
            duration={medVal}
          />
          <Value 
            ruleName="longVal"            
            onChange={ newVal => this.onChangeDuration('longVal', newVal)}
            descriptionText={longValDesc}
            hintText="measured in hourly intervals"
            duration={longVal}
          />
        </form>
        <div className="m-t">                          
          <Button
            size="sm"
            className="btn-primary"
            isProcessing={isProcessing}
            isDisabled={!isDirty}
            onClick={this.onSave}>              
            Save
          </Button>
        </div>
      </Box>
    )
  }
})

const RetentionValuesContainer = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      retentionValues: getters.retentionValues,
      updateAttemp: getters.updateLogFwrAttemp
    }
  },
  
  onSave(defVal, medVal, longVal){
    saveRetentionValues(
      fromMsToNano(defVal), 
      fromMsToNano(medVal), 
      fromMsToNano(longVal));
  },

  render(){
    let { updateAttemp, retentionValues } = this.state;
    let { defVal, medVal, longVal } = retentionValues; 

    defVal = fromNanoToMs(defVal);
    medVal = fromNanoToMs(medVal);
    longVal = fromNanoToMs(longVal);

    let props = {  
      ...updateAttemp,                   
      defVal,
      medVal,
      longVal,                      
      onSave: this.onSave             
    }
          
    return ( <RetentionValues { ...props }/> )
  }

});

export default RetentionValuesContainer;