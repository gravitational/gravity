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
import 'brace/mode/yaml';
import 'brace/mode/json';
import 'brace/ext/searchbox';
import 'brace/theme/monokai';
import StyledTextEditor from './StyledTextEditor';

const { UndoManager } = ace.acequire('ace/undomanager');

class TextEditor extends React.Component{

  onChange = () => {
    let value = this.session.getValue();
    if (this.props.onChange) {
      this.props.onChange(value);
    }
  }

  componentDidUpdate(){
    this.editor.resize();
  }

  initEditSessions(data, readOnly) {
    let undoManager = new UndoManager();
    data = data || '';
    const mode = getMode(this.props.docType);
    this.isDirty = false;
    this.session = new ace.EditSession(data);
    this.session.setOptions({ tabSize: 2, useSoftTabs: true });
    this.session.setUndoManager(undoManager);
    this.session.setUseWrapMode(false)
    this.session.setMode(mode);
    this.editor.setTheme('ace/theme/monokai');
    this.editor.setSession(this.session);
    this.editor.setReadOnly(readOnly);
  }

  componentDidMount() {
    const { data, readOnly } = this.props;
    this.editor = ace.edit(this.ace_viewer);
    this.editor.setFadeFoldWidgets(true);
    this.editor.setWrapBehavioursEnabled(true);
    this.editor.setHighlightActiveLine(false);
    this.editor.setShowInvisibles(false);
    this.editor.renderer.setShowGutter(false);
    this.editor.renderer.setShowPrintMargin(false);
    this.editor.renderer.setShowGutter(true);
    this.editor.on('input', this.onChange);
    this.initEditSessions(data, readOnly);
    this.editor.focus();
  }

  componentWillUnmount() {
    this.editor.destroy();
    this.editor = null;
    this.session = null;
  }

  // to properly recalculate scrollbar area when layout is changed
  shouldComponentUpdate() {
    return true;
  }

  render() {
    return (
      <StyledTextEditor>
        <div ref={e => this.ace_viewer = e} />
      </StyledTextEditor>
    )
  }
}

export default TextEditor;

function getMode(docType){
  if( docType === 'json' ){
    return 'ace/mode/json';
  }

  return 'ace/mode/yaml';
}