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
import ace from 'brace';
import 'brace/mode/yaml';
import 'brace/ext/searchbox';
//import 'brace/theme/iplastic';

const { UndoManager } = ace.acequire('ace/undomanager');

class YamlEditor extends React.Component{

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
    this.isDirty = false;
    this.session = new ace.EditSession(data);      
    this.session.setOptions({ tabSize: 2, useSoftTabs: true });
    this.session.setUndoManager(undoManager);
    this.session.setUseWrapMode(false)        
    this.session.setMode("ace/mode/yaml");  
    //this.editor.setTheme('ace/theme/iplastic');
    this.editor.setSession(this.session);
    this.editor.setReadOnly(readOnly);        
  }
  
  componentDidMount() {
    const { data, readOnly } = this.props;
    this.editor = ace.edit(this.refs.ace_viewer);            
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
      <div style={mainStyle}>
        <div className="grv-settings-json-editor">            
          <div ref="ace_viewer" style={editorStyle}></div>                  
        </div>                        
      </div>                  
    )
  }
}

const mainStyle = {    
  width: "100%",
  overflow: "auto",
  flex: "1",
  display: "flex",
  position: "relative",
  border: "1px solid #e7eaec",
  padding: "2px"
}

const editorStyle = {
  position: 'absolute',
  top: '0px',
  right: '0px',
  bottom: '0px',
  left: '0px'
};

export default YamlEditor;
