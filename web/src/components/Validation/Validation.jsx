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

import React from 'react';
import Logger from 'app/lib/logger';
import { values } from 'lodash';

const logger = Logger.create('validation');

// Validator handles input validation
export default class Validator {

  errors = {}

  constructor(){
    // store subscribers
    this._subs = [];
  }

  // adds a callback to the list of subscribers
  subscribe(cb){
    this._subs.push(cb)
  }

  // removes a callback from the list of subscribers
  unsubscribe(cb){
    const index = this._subs.indexOf(cb);
    if (index > -1) {
      this._subs.splice(index, 1)
    }
  }

  addError(name, state){
    this.errors[name] = state;
    logger.info(`${name}`, this.errors)
  }

  isValid(){
    const errors = this.validate();
    return !values(errors).some(value => !!value);
  }

  reset(){
    this.errors = {};
    this.validating = false;
  }

  validate() {
    this.reset();
    this.validating = true;
    this._subs.forEach(cb => {
      try{
        cb();
      }
      catch(err){
        logger.error(err);
      }
    })

    return this.errors;
  }
}

const ValidationProviderContext =  React.createContext({});

export function ValidationContext(props) {
  const [ validator ] = React.useState(() => new Validator() );
  return (
    <ValidationProviderContext.Provider value={validator} children={props.children} />
  )
}

export function useValidationContext(){
  const value = React.useContext(ValidationProviderContext);
  if(!(value instanceof Validator)){
    logger.warn('Missing Validation Context declaration')
  }

  return value;
}

/**
 * useError registeres with validation context and runs validation function
 * after validation has been requested
 */
export function useError(name, validate){
  if (!name) {
    logger.warn(`useError("${name}", fn), error name cannot be empty`);
    return;
  }

  if (typeof validate !== "function") {
    logger.warn(`useError("${name}", fn), fn() must be a function`);
    return;
  }

  const [ , rerender ] = React.useState({});
  const validator = useValidationContext();

  // register to validation context to be called on validate()
  React.useEffect(() => {
    function onValidate(){
      if(validator.validating){
        const errorState = validate();
        validator.addError(name, errorState);
        rerender();
      }
    }

    // subscribe to store changes
    validator.subscribe(onValidate);

    // unsubscribe on unmount
    function cleanup(){
      validator.unsubscribe(onValidate)
    }

    return cleanup;
  }, [validate]);

  // if validation has been requested, validate right away.
  if(validator.validating){
    return validate();
  }

  return null;
}
