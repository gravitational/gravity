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

var GrvDialogDefaultHeader = ({title}) => <h2 className="m-t-xs">{title}</h2>;

export const GrvDialogHeader = React.createClass({
  render(){
    return ( <div>{this.props.children}</div> )
  }
});

export const GrvDialogContent = React.createClass({
  render(){
    return ( <div>{this.props.children}</div> )
  }
});

export const GrvDialogFooter = React.createClass({
  render(){
    let {onClose, children} = this.props;
    let $content = children;
    if(!children){
      $content = (
        <button onClick={onClose} ref="cancel" type="button" className="btn btn-white">
          Close
        </button>
      )
    }

    return (
      <div className="modal-footer">
        { $content }
      </div>
    )
  }
});

export const GrvDialog = React.createClass({

  componentWillUnmount(){
    $(this.refs.modal).modal('hide');
  },

  componentDidMount(){
    $(this.refs.modal).modal('show');
  },

  render() {
    let { title, onClose, className='' } = this.props;
    let $header = <GrvDialogDefaultHeader title={title} />
    let $footer = <GrvDialogFooter onClose={onClose}/>;
    let $content = null;

    className = `modal inmodal grv-dialog ${className}`;

    React.Children.forEach(this.props.children, (child) => {
      if (child == null) {
        return;
      }

      if(child.type.displayName === 'GrvDialogFooter'){
        $footer = child;
      }

      if(child.type.displayName === 'GrvDialogContent'){
        $content = (
          <div className="modal-body">
            <div className="grv-dialog-content">
              {child}
            </div>
          </div>
        )
      }

      if(child.type.displayName === 'GrvDialogHeader'){
        $header = child;
      }
    });

    return (
      <div ref="modal" className={className} data-keyboard="false" data-backdrop="static" tabIndex={-1} role="dialog">
        <div className="modal-dialog">
          <div className="modal-content">
            <div className="modal-header grv-dialog-content-header">
              {$header}
            </div>
            {$content}
            {$footer}
          </div>
        </div>
      </div>
    );
  }
});