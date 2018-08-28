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
import Button from 'app/components/common/button';

export const Separator = () => (<div className="grv-line-solid m-t-lg m-b-lg"></div>);

export const Form = React.createClass({

  refCb(e) {    
    if (this.props.refCb) {
      this.props.refCb(e);
    }
  },

  onSubmit(e) {    
    e.preventDefault();
    return false;
  },

  render() {    
    let { className='', style = {}, children } = this.props;
    return (
      <form ref={this.refCb}
        className={className}
        style={style}
        onSubmit={this.onSubmit}>
        {children}
      </form>
    );
  }
});

export const NewButton = props => (
  <Button
    size="sm"
    isDisabled={!props.enabled}
    onClick={props.onClick}
    className="grv-settings-res-new m-t btn-default">
    <i className="fa fa-plus m-r-xs"/>{props.text}
  </Button>
)

export const EmptyList = ({canCreate=true, onClick}) => (  
  <div className="text-center" style={{ minHeight: "50px", margin: "25px auto", maxWidth: "600px" }}>
    <p className="no-margins">
      <strong>You do not have anything here</strong>
    </p>      
    <NewButton enabled={canCreate}text="Create" onClick={onClick}/>                  
  </div>      
)