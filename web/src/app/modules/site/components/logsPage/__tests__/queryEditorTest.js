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
import * as ReactDOM from 'react-dom';
import expect from 'expect';
import QueryEditor from 'app/modules/site/components/logsPage/queryEditor';
import { $ } from 'app/__tests__/';
import { makeHelper } from 'app/__tests__/domUtils';

const $node = $('<div>');
const helper = makeHelper($node);

const suggestions = [
  {
    type: 'pod',
    text: 'pod1',
    details: {
      podHostIp: 'pod1_host',
      podIp: 'pod1_ip'
    }
  },
  {
    type: 'pod',
    text: 'pod2',
    details: {
      podHostIp: 'pod1_host',
      podIp: 'pod1_ip'
    }
  },
  {
    type: 'container',
    text: 'container1',
    details: {
      podHostIp: 'container1_host',
      podIp: 'container1_ip'
    }
  }
]

describe('app/modules/site/components/logsPage/queryEditor', () => {    

  beforeEach(() => {          
    helper.setup();    
  })

  afterEach(() => {    
    helper.clean();    
  })
      
  it('should handle passing props', () => {      
    let wasCalled = false;
    const query = 'hi'
    const inputValue = 'bye';
    const onChange = value => {
      expect(value).toMatch(inputValue)
      wasCalled = true;
    }
        
    render(<QueryEditor query={query} suggestions={suggestions} onChange={onChange} />);            
    const input = $node.find('input')[0]    
    expect(input.value).toBe(query);        
    
    helper.setText(input, inputValue);                  
    helper.keyDown(input, { key: "Enter", keyCode: 13, which: 13 });              
    expect(wasCalled).toBe(true);    
  });        

  it('should show pod options', () => {                                                                 
    const cmpt = render(<QueryEditor suggestions={suggestions} />);                            
    cmpt.append("pod:")        
    expect($node[0].querySelectorAll('.grv-input-autocomplete-list-item').length).toBe(2)        
  });        

  it('should show container options', () => {                                                                 
    const cmpt = render(<QueryEditor suggestions={suggestions} />);                            
    cmpt.append("container:")        
    expect($node[0].querySelectorAll('.grv-input-autocomplete-list-item').length).toBe(1)        
  });        
  
  it('should filter options', () => {                                                                 
    const cmpt = render(<QueryEditor suggestions={suggestions} />);                            
    cmpt.append("pod:pod2")        
    expect($node[0].querySelectorAll('.grv-input-autocomplete-list-item').length).toBe(1)        
  });        

  it('should insert option text on press enter', () => {                                                                 
    const cmpt = render(<QueryEditor suggestions={suggestions} />);                            
    const input = $node.find('input')[0]    
    cmpt.append("pod:")        
    helper.keyDown(input, { key: "Enter", keyCode: 13, which: 13 });              

    helper.shouldNotExist('.grv-input-autocomplete-list-item');
    expect(input.value).toBe('pod:pod1 ')    
  });          
});

const render = cmpt => {      
  return ReactDOM.render(cmpt, $node[0]);    
}
