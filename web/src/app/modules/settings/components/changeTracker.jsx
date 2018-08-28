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
import ConfirmChangesDialog from './confirmChangesDialog';

class ChangeTracker extends React.Component {

  constructor(props) {
    super(props);          
    this._unsubscribe = () => false;
    this._dirtyChildren = [];
    this.state = {
      isConfirmDialogVisiable: false,
      onConfirmDialogOk: () => { }      
    }
  }

  static childContextTypes = {
    changeTracker: React.PropTypes.object.isRequired  
  }

  static contextTypes = {
    router: React.PropTypes.object.isRequired      
  };

  register = instance => {
    this._dirtyChildren.push(instance);
  }

  unregister = instance => {
    let index = this._dirtyChildren.indexOf(instance);
    if (index !== -1) {
      this._dirtyChildren.splice(index, 1);
    }
  }

  onCloseConfirmDialog = () => {
    this.setState({ isConfirmDialogVisiable: false });
  }

  onRouterWillLeave = nextLocation => {
    if (nextLocation.state && nextLocation.state.ignore === true) {
      return true;
    }

    return !this.checkIfUnsafedData(() => {
      nextLocation.state = { ignore: true };
      setTimeout(() => this.context.router.push(nextLocation), 0);
    });
  }
  
  getChildContext() {
    return {
      changeTracker: this      
    };
  }

  componentDidMount() {    
    this._unsubscribe = this.context.router.setRouteLeaveHook(
      this.props.route,
      this.onRouterWillLeave);
  }

  componentWillUnmount(){
    this._unsubscribe();
    this._dirtyChildren = [];
  }
          
  checkIfUnsafedData(cb) {
    let hasChanges = this._dirtyChildren.some(inst => inst.hasChanges());

    if (hasChanges) {
      let wrapperCb = () => {
        this.onCloseConfirmDialog();
        cb();
      }

      this.setState({
        isConfirmDialogVisiable: true,
        onConfirmDialogOk: wrapperCb
      })
    } else {
      cb();
    }

    return hasChanges;
  }

  render() {    
    const { isConfirmDialogVisiable, onConfirmDialogOk } = this.state;
    const className = this.props.className || "";
    return (
      <div className={className}>
        {this.props.children}
        <ConfirmChangesDialog
          isVisible={isConfirmDialogVisiable}
          onOk={onConfirmDialogOk}
          onCancel={this.onCloseConfirmDialog} />
      </div>
    );
  }
}
  
export default ChangeTracker;