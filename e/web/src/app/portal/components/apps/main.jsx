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
