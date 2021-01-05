/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import classnames from 'classnames';
import WhatIs from 'oss-app/components/whatis'
import { UserFilterEnum, VersionFilterEnum } from './../enums';
import AppList from './appList';

const Apps = React.createClass({

  getInitialState(){
    return {
      userFilter: UserFilterEnum.EVERYONE,
      verFilter: VersionFilterEnum.LATEST,
    }
  },

  onUserFilterClick(userFilter){
    this.setState({
      userFilter
    })
  },

  onVerFilterClick(verFilter){
    this.setState({
      verFilter
    })
  },

  getBtnClass(isActive){
    return classnames('btn btn-sm', {
      'btn-white': !isActive,
      'btn-white active': isActive,
    })
  },

  getVerBtnClass(filter){
    return this.getBtnClass(this.state.verFilter === filter)
  },

  getUserBtnClass(filter){
    return this.getBtnClass(this.state.userFilter === filter)
  },

  render() {
    let { userFilter, verFilter } = this.state;
    return (
      <div>
        <div className="grv-portal-apps-toolbar m-b-sm">
          <h3 className="no-margins">
            <span>Application Bundles </span>
            <WhatIs.OpsCenterApps className="grv-portal-whatis-trigger"/>
          </h3>
          <div className="grv-portal-apps-toolbar-controls">
            <div className="">
              <label className="m-r-sm">Versions</label>
              <div className="btn-group">
                <button type="button" onClick={ () => this.onVerFilterClick(VersionFilterEnum.ALL ) } className={ this.getVerBtnClass(VersionFilterEnum.ALL) }>All</button>
                <button type="button" onClick={ () => this.onVerFilterClick(VersionFilterEnum.LATEST) } className={ this.getVerBtnClass(VersionFilterEnum.LATEST) }>Latest</button>
              </div>
            </div>
          </div>
        </div>
        <div className="grv-portal-list">
          <AppList userFilter={userFilter} verFilter={verFilter}/>
        </div>
      </div>
    )
  }
});

export default Apps;
