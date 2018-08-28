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
import classnames from 'classnames';
import WhatIs from 'app/components/whatis';

class AppTags extends React.Component {

  static propTypes = {
   tags: React.PropTypes.object.isRequired,
   onChange: React.PropTypes.func.isRequired
  }
  
  constructor(props) {
    super(props);
    this.state = {      
      newTagKey: '',
      newTagValue: ''
    }
  }
    
  onAddTag = e => {
    e.preventDefault();
    let { tags, onChange } = this.props;
    let { newTagKey, newTagValue } = this.state;

    if(!newTagKey || !newTagValue){
      return;
    }

    tags[newTagKey] = newTagValue;

    onChange(tags);

    this.setState({
      newTagKey: '',
      newTagValue: ''
    });

    this.refs.tagKey.focus();
  }

  onDeleteTag(key){
    let { tags, onChange } = this.props;
    delete tags[key];
    onChange(tags);
  }

  renderTable(){
    let { tags } = this.props;
    if (Object.getOwnPropertyNames(tags).length === 0){
      return null;
    }

    return (
      <table className="table m-t-xs m-b-sm">
        <colgroup>
          <col width="41.66667%"/>
          <col width="41.66667%" />
        </colgroup>
        {this.renderTableBody()}
      </table>
    )
  }

  renderTableBody(){
    let { tags } = this.props;
    let $rows = Object.getOwnPropertyNames(tags).map( key => {
      let value = tags[key];
      return (
        <tr key={key}>
          <td>{key}</td>
          <td>{value}</td>
          <td className="text-right">
            <a className="btn btn-xs btn-white" onClick={this.onDeleteTag.bind(this, key)}>
              <i className="fa fa-trash"></i> <span></span>
            </a>
          </td>
        </tr>
      )
    })

    return <tbody>{$rows}</tbody>
  }

  render(){    
    let { newTagKey, newTagValue } = this.state;

    let btnClass = classnames('btn btn-white pull-right', {
      'disabled' : !newTagKey || !newTagValue
    });

    return (
      <div>        
        <label>
          <span>Tags </span>
          <WhatIs.Tags placement="top"/>
        </label>
        {this.renderTable()}
        <div className="row">
          <form className="form">
            <div className="form-group col-sm-6">
              <input                
                value={newTagKey}
                onChange={e => this.setState({ newTagKey: e.target.value })}
                name="tagKey"
                ref="tagKey"
                className="form-control" placeholder="Enter key"
                />
            </div>
            <div className="form-group col-sm-5">
              <input
                value={newTagValue}
                onChange={e => this.setState({ newTagValue: e.target.value })}                  
                name="tagValue"
                ref="tagValue"
                className="form-control" placeholder="Enter value"
                />
            </div>
            <div className="form-group col-sm-1">
              <div>
                <a href="#" className={btnClass} onClick={this.onAddTag}>
                  <i className="fa fa-plus"></i>
                </a>
              </div>
            </div>
          </form>
        </div>
      </div>
    )
  }    
}

export default AppTags;