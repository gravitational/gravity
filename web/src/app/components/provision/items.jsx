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
import htmlUtils from 'app/lib/htmlUtils';

export const ServerListLabel = ({text, tooltip}) => (
  <label>{text}
    <If visible={tooltip}>
      <i title={tooltip} className="fa fa-question-circle m-l-xs" aria-hidden="true"></i>
    </If>
  </label>
)


const If = ({visible, children}) => visible ? children : null;

export const ServerInstructions = React.createClass({

  onCopyClick(textToCopy, event){
    event.preventDefault();
    htmlUtils.copyToClipboard(textToCopy);
    htmlUtils.selectElementContent(this.refs.command);
  },

  render: function() {
    let {text} = this.props;
    let containerStyle={
      "alignItems": "center",
      "display": "flex"
    }

    let textStyle = {
      resize: 'none',
      overflow: 'hidden',
      display: 'block',
      padding: '9.5px',
      width: '100%',
      fontSize: '13px',
      lineHeight: '1.42857',
      wordBreak: 'break-all',
      wordWrap: 'break-word',
      color: '#333333',
      backgroundColor: '#f5f5f5',
      border: '1px solid #ccc',
      borderRadius: '4px'
    }

    return (
      <div className="grv-installer-server-instruction p-xs text-muted">
        <strong>Add existing server</strong>
        <div>Copy and paste the command below into terminal. Your server will automatically appear in the list.</div>
        <div style={containerStyle} className="m-t-sm">
          <span ref="command" style={textStyle} className="form-conrol" defaultValue={text}>{text}</span>
          <button onClick={this.onCopyClick.bind(this, text)} className="btn btn-sm btn-primary m-l">Copy Command</button>
        </div>
      </div>
    );
  }
});