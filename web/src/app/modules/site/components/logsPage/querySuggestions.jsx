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
import ReactDOM from 'react-dom';
import classnames from 'classnames';

class Suggestions extends React.Component {

  onMouseClick = e => {    
    e.preventDefault();
    const element = e.target;
    const children = element.parentNode.children;
    for (let i = 0; i < children.length; i++){
      if (children[i] === element) {        
        this.props.onMouseClick(i);    
      }
    }        
  }

  renderPod(data){
    const { text, details } = data;
    const { podHostIp, podIp } = details;
    return (
      <div>
        <div>{text}</div>
        <div className="text-muted">
          <small>host-ip: {podHostIp}</small> / <small>pod-ip: {podIp}</small>
        </div>
      </div>
    )
  }

  componentDidUpdate() {
    if(!this.refs.popupMenu){
      return;
    }

    const [ activeItem ] = this.refs.popupMenu.getElementsByClassName('--active');
    if(activeItem){  
      // scroll
      const focusedDOM = ReactDOM.findDOMNode(activeItem);
      const menuDOM = ReactDOM.findDOMNode(this.refs.popupMenu);
      const focusedRect = focusedDOM.getBoundingClientRect();
      const menuRect = menuDOM.getBoundingClientRect();
      if (focusedRect.bottom > menuRect.bottom || focusedRect.top < menuRect.top) {
        menuDOM.scrollTop = (focusedDOM.offsetTop + focusedDOM.clientHeight - menuDOM.offsetHeight);
      }    
    }  
   }

  renderItems(){
    const { curItem, data } = this.props;        
    let $items = data.map((dataItem, index) => {
      const { text, /* type */ } = dataItem;
      const itemClass = classnames('grv-input-autocomplete-list-item', {
        '--active': index === curItem
      });
      
      return (
        <li key={index} className={itemClass} onClick={this.onMouseClick}>
          {text}
        </li>
      )
    });

    if($items.length === 0){
      $items = (
        <li className="grv-input-autocomplete-list-item">
          <span className="text-muted">No suggestions</span> 
        </li>
      )
    }

    return $items;
  }

  render(){
    const $items = this.renderItems();
    if( $items.length === 0){
      return null;
    }

    return (
      <ul ref="popupMenu" className="grv-input-autocomplete-list">
        {$items}
      </ul>
    );
  }
}

export default Suggestions;