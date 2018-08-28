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
import {Select} from 'app/components/common/dropDown';
import Button from 'app/components/common/button';
import reactor from 'app/reactor';
import cfgMapsGetters from './../../flux/k8sConfigMaps/getters';
import * as cfgMapsActions from './../../flux/k8sConfigMaps/actions';
import ConfigEditor from './configEditor';
import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from 'app/components/dialogs/dialog';

const ConfigPage = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      model: cfgMapsGetters.configMaps,
      updateAttemp: cfgMapsGetters.updateCfgMapAttemp      
    }
  },

  getInitialState(){
    return {
      isConfirmDialogVisiable: false,
      onConfirmDialogOk: () => {}
    }
  },

  onChangeNamespace(value){
    this._checkIfUnsafedData(()=> cfgMapsActions.setSelectedNamespace(value));
  },

  onChangeConfigMap(value){
    this._checkIfUnsafedData(()=> cfgMapsActions.setSelectedCfgName(value));
  },

  onCloseConfirmDialog(){
    this.setState({
      isConfirmDialogVisiable: false,
      onConfirmDialogOk: ()=> {}
    })
  },

  onRouterWillLeave(nextLocation) {
    return !this._checkIfUnsafedData(()=> {
      setTimeout( ()=> this.context.router.push(nextLocation), 0);
    });
  },

  onApplyChanges(){
    let updatedData = {};
    let changes = this.refs.configEditor.getChanges();
    let {
      selectedConfigName,
      selectedNamespaceName} = this.state.model;

    changes.data.forEach(item=> updatedData[item.name] = item.content);
    cfgMapsActions.saveConfigMap(selectedNamespaceName, selectedConfigName, updatedData);
  },

  componentDidMount() {      
    this.context.router.setRouteLeaveHook(this.props.route, this.onRouterWillLeave)
  },

  render() {
    let {
      updateAttemp,      
      model
    } = this.state;

    let {
      isDirty,
      configs,
      namespaceNames,
      selectedConfigName,
      selectedNamespaceName
    } = model;
    
    let isUpdating = updateAttemp.isProcessing;        
    let { isConfirmDialogVisiable, onConfirmDialogOk } = this.state;
    let namespaceOptions = namespaceNames.map(item => ({value: item, label: item}));
    let configNameOptions = configs.map(item => ({value: item.name, label: item.name}));
    let selectedConfigMap = configs.find(item => item.name === selectedConfigName);
    let $content = null;

    if( selectedConfigMap ){
      $content = [
        <ConfigEditor key={0} ref="configEditor" {...selectedConfigMap} isDirty={isDirty} onDirty={cfgMapsActions.makeDirty}/>,
        <div key={1} className="m-t-xs m-b-xs">
          <Button
            className="btn-primary"
            isPrimary={false} onClick={this.onApplyChanges}
            isProcessing={isUpdating}
            isDisabled={!isDirty}>
            Apply
          </Button>
        </div>
      ]
    }else{
      $content = (
         <div className="grv-site-configs-hints p-sm m-b-xs">
          <h2 className="text-muted" style={{alignSelf: 'center', margin: '0 auto'}}>Please select a config map</h2>
        </div>
      )
    }

    return (
      <div className="grv-site-configs grv-page">
        <ConfirmDialog
          isVisible={isConfirmDialogVisiable}
          onOk={onConfirmDialogOk}
          onCancel={this.onCloseConfirmDialog}/>
        <div className="grv-site-configs-header m-b-sm">
          <div>
            <label>Config maps</label>
            <Select
              onChange={this.onChangeConfigMap}
              value={selectedConfigName}
              options={configNameOptions}/>
          </div>
          <div>
            <label>Namespace</label>
            <Select
              onChange={ this.onChangeNamespace }
              value={selectedNamespaceName}
              options={namespaceOptions}/>
          </div>
        </div>
        {$content}
      </div>
    )
  },

  _checkIfUnsafedData(cb){
    let { isDirty } = this.state.model;

    if(isDirty){
      let wrapperCb = ()=> {
        cfgMapsActions.makeDirty(false);
        this.onCloseConfirmDialog();
        cb();
      }

      this.setState({
        isConfirmDialogVisiable: true,
        onConfirmDialogOk: wrapperCb
      })
    }else{
      cb();
    }

    return isDirty;
  }

});

ConfigPage.contextTypes = {
  router: React.PropTypes.object.isRequired
}

const ConfirmDialog = React.createClass({
  render(){
    let { isVisible, onOk, onCancel } = this.props;

    if(!isVisible){
      return null;
    }

    return (
      <GrvDialog title="" className="grv-dialog-no-body grv-site-configs-dlg-confirm" >
        <GrvDialogHeader>
          <div className="grv-site-configs-dlg-confirm-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className="fa fa-exclamation-triangle fa-2x text-warning" aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">You have unsaved changes!</h3>
              <small>If you navigate away you will lose your unsaved changes.</small>
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogFooter>
          <Button onClick={onOk} className="btn-warning">
            Disregard and continue
          </Button>
          <Button
            onClick={onCancel}
            isPrimary={false}
            className="btn btn-white">
            Close
          </Button>
        </GrvDialogFooter>
      </GrvDialog>
    )
  }
})

export default ConfigPage;
