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
import ace from 'brace';
import 'brace/ext/searchbox';

const { UndoManager } = ace.acequire('ace/undomanager');

var ConfigEditor = React.createClass({

  getInitialState() {
    return {
      activeTabIndex: 0,
      dirtyTabs: []
    }
  },

  getChanges(){
    let newData = [];
    let {data, id} = this.props;

    this._ensureEditor( () => {
      let newContent = this.refs.editor.getContent();
      newData = data.map( (item, key) => ({
        name: item.name,
        content: newContent[key]
      }));
    });

    return {
      data: newData,
      id
    }
  },

  onChangeTab(activeTabIndex) {
    this.setState({activeTabIndex})
  },

  makeTabDirty(index, value){
    let { data=[] } = this.props;
    let { dirtyTabs } = this.state;
    dirtyTabs[index] = value;

    this.setState({
      dirtyTabs
    })

    let isDirty = data.some((item, index) => dirtyTabs[index]);

    this.props.onDirty(isDirty);
  },

  componentWillReceiveProps(newProps) {
    let {id, isDirty} = this.props;
    if(newProps.id !== id){
      this.setState(this.getInitialState());
    }else if(newProps.isDirty === false && isDirty === true){
      this._clearDirtyFlaga();
    }
  },

  _clearDirtyFlaga(){
    this._ensureEditor( () => this.refs.editor.clearUndoManager() );
    this.setState({
      ...this.state,
      dirtyTabs: []
    })
  },

  _ensureEditor(f){
    if(this.refs.editor){
      f();
    }
  },

  renderHeader(item, key) {
    let {name} = item;
    let itemClass = classnames({
      'active': key === this.state.activeTabIndex
    })

    let itemIconClass = classnames('', {
      'hidden': this.state.dirtyTabs[key] !== true
    })

    return (
      <li key={key} className={itemClass}>
        <a href="#" onClick={() => this.onChangeTab(key)}>{name} <span style={{fontSize: '10px'}} className={itemIconClass}>*</span></a>
      </li>
    )
  },

  render() {
    let { data = [], id, name} = this.props;
    let $headerItems = data.map(this.renderHeader);
    let isEmpty = data.length === 0;

    let editorProps = {
      id,
      onDirty: this.makeTabDirty,
      initialData: data,
      activeIndex: this.state.activeTabIndex
    };

    return (
      <div className="grv-site-configs-editor-box p-xs m-b-xs">
        {
          isEmpty ? <h2 className="text-muted" style={{alignSelf: 'center', margin: '0 auto'}}>{name} is empty</h2>
          :
          <div className="grv-site-configs-editor tabs-container">
            <ul className="nav nav-tabs">
              {$headerItems}
            </ul>
            <div className="tab-content">
              <div className="tab-pane active">
                <div className="panel-body">
                  <Editor ref="editor" {...editorProps}/>
                </div>
              </div>
            </div>
          </div>
       }
      </div>
    )
  }
});

const editorStyle = {
  position: 'absolute',
  top: '0px',
  right: '0px',
  bottom: '0px',
  left: '0px'
};

let Editor = React.createClass({

  clearUndoManager(){
    this.sessions.forEach(s => s.getUndoManager().markClean() );
  },

  _createSession(content) {
    let session = new ace.EditSession(content);
    let undoManager = new UndoManager();

    undoManager.markClean();
    session.setUndoManager(undoManager);
    session.setUseWrapMode(true)
    return session;
  },

  getContent(){
    return this.sessions.map(s => s.getValue());
  },

  setActiveSession(index) {
    let activeSession = this.sessions[index];
    if (!activeSession) {
      activeSession = this._createSession('');
    }

    this.editor.setSession(activeSession);
  },

  initEditSessions(data=[]) {
    this.isDirty = false;
    this.sessions = data.map(item => this._createSession(item.content));
    this.setActiveSession(0);
  },

  onChange(){
    let isClean = this.editor.session.getUndoManager().isClean();
    if(this.props.onDirty){
      this.props.onDirty(this.props.activeIndex, !isClean);
    }
  },

  componentDidMount() {
    this.editor = ace.edit(this.refs.ace_viewer);
    this.editor.renderer.setShowGutter(false);
    this.editor.renderer.setShowPrintMargin(false);
    this.editor.setWrapBehavioursEnabled(true);
    this.editor.setHighlightActiveLine(true);
    this.editor.setShowInvisibles(false);
    this.editor.setReadOnly(false);
    this.editor.on('input', this.onChange);
    this.initEditSessions(this.props.initialData);
  },

  componentWillUnmount() {
    this.editor.destroy();
    this.editor = null;
    this.sessions = [];
  },

  componentWillReceiveProps(newProps) {
    if(newProps.id !== this.props.id){
      this.initEditSessions(newProps.initialData);
    }else if(newProps.activeIndex !== this.props.activeIndex){
      this.setActiveSession(newProps.activeIndex);
    }
  },

  shouldComponentUpdate() {
    return false;
  },

  render() {
    return (
      <div ref="ace_viewer" style={editorStyle}></div>
    )
  }
});

export default ConfigEditor;
