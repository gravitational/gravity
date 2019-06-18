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
import ace from 'brace';
import StyledTextEditor from 'app/components/TextEditor/StyledTextEditor';

import 'brace/mode/yaml';
//import 'brace/mode/json';
import 'brace/ext/searchbox';
import 'brace/theme/monokai';

const { UndoManager } = ace.acequire('ace/undomanager');

export default class Editor extends React.Component {

  clearUndoManager(){
    this.sessions.forEach(s => s.getUndoManager().markClean() );
  }

  createSession(content) {
    let session = new ace.EditSession(content);
    let undoManager = new UndoManager();

    undoManager.markClean();
    session.setUndoManager(undoManager);
    session.setUseWrapMode(true)
    session.setMode('ace/mode/yaml');
    return session;
  }

  getContent(){
    return this.sessions.map(s => s.getValue());
  }

  setActiveSession(index) {
    let activeSession = this.sessions[index];
    if (!activeSession) {
      activeSession = this.createSession('');
    }

    this.editor.setSession(activeSession);
  }

  initEditSessions(data=[]) {
    this.isDirty = false;
    this.sessions = data.map(item => this.createSession(item.content));
    this.setActiveSession(0);
  }

  onChange(){
    let isClean = this.editor.session.getUndoManager().isClean();
    if(this.props.onDirty){
      this.props.onDirty(this.props.activeIndex, !isClean);
    }
  }

  componentDidMount() {
    this.editor = ace.edit(this.aceViewerRef);
    this.editor.setTheme('ace/theme/monokai');
    this.editor.renderer.setShowGutter(false);
    this.editor.renderer.setShowPrintMargin(false);
    this.editor.setWrapBehavioursEnabled(true);
    this.editor.setHighlightActiveLine(true);
    this.editor.setShowInvisibles(false);
    this.editor.setReadOnly(false);
    this.editor.on('input', this.onChange.bind(this));
    this.initEditSessions(this.props.initialData);
  }

  componentWillUnmount() {
    this.editor.destroy();
    this.editor = null;
    this.sessions = [];
  }

  componentDidUpdate(prevProps){
    if(prevProps.activeIndex !== this.props.activeIndex){
      this.setActiveSession(this.props.activeIndex);
    }
  }

  render() {
   return (
    <StyledTextEditor>
      <div ref={e => this.aceViewerRef = e} />
    </StyledTextEditor>
    )
  }
}
