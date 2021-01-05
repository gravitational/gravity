import SiteList from './siteList';
import React from 'react';
import classnames from 'classnames';

// oss
import reactor from 'oss-app/reactor';
import WhatIs from 'oss-app/components/whatis'
import userGetters from 'oss-app/flux/user/getters';
import * as userAclFlux from 'oss-app/flux/userAcl';
import {isMatch } from 'oss-app/lib/objectUtils';
import AjaxPoller from 'oss-app/components/dataProviders';

import { EmptyList } from './../items.jsx';
import SearchQuery from './siteListFilterInput.jsx';
import { UserFilterEnum } from './../enums';
import * as actions from './../../flux/sites/actions';
import getters from './../../flux/sites/getters';

const NO_MATCH_TEXT = 'No matching clusters found';
const NO_DATA_TEXT = 'You have no clusters';

const PortalSites = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      user: userGetters.user,
      userAclStore: userAclFlux.getters.userAcl,
      siteInfos: getters.sitesInfo
    }
  },

  getInitialState(){
    this.defaultSearchableProps = [
      SearchablePropEnum.APP,
      SearchablePropEnum.DOMAIN,
      SearchablePropEnum.CREATED_BY
    ];

    return {
      userFilter: UserFilterEnum.EVERYONE,
      queryInfo: {
        labels: {},
        text: ''
      }
    }
  },

  onUserFilterClick(userFilter){
    this.setState({
      userFilter
    })
  },

  onChangeSearchValue(queryInfo){
    this.setState({
      queryInfo
    })
  },

  doSearch(data){
    let { queryInfo } = this.state;
    let { text, labels } = queryInfo;
    let labelKeys = Object.keys(labels);

    // first do deafult filtering
    data = data.filter(obj => isMatch(obj, text, {
      searchableProps: this.defaultSearchableProps
    }));

    if(labelKeys.length > 0){
      data = data.filter(obj => {
        let objLabels = obj[SearchablePropEnum.LABELS];
        let shouldKeep = labelKeys.every(key => {
          if(builtInLabels[key]){
            return doesMatch(obj[builtInLabels[key]], labels[key]);
          }

          if(objLabels[key] === undefined){
            return false;
          }

          return doesMatch(objLabels[key], labels[key]);
        });

        return shouldKeep;
      })
    }

    return data;
  },

  doFilter(data){
    let { user, userFilter } = this.state;
    if(userFilter === UserFilterEnum.ME){
      return data.filter(item => item.createdBy === user.userId);
    }

    return data;
  },

  getAllLabels(data){
    let allLabels = {
      ...builtInLabels
    };

    data.forEach(item => {
      Object.keys(item[SearchablePropEnum.LABELS])
            .forEach(key => allLabels[key] = 1);
    })

    return [ ...Object.keys(allLabels ) ].sort();
  },

  getUserBtnClass(filter){
    let isActive = this.state.userFilter === filter;
    return classnames('btn btn-sm', {
      'btn-white': !isActive,
      'btn-white active': isActive,
    })
  },

  render() {
    let { siteInfos=[], userAclStore, userFilter } = this.state;
    let allLabels = [];
    let $data = null;

    if (!userAclStore.getClusterAccess().list) {
      return null;
    }

    if(siteInfos.length > 0){
      let data = this.doFilter(siteInfos);
      // get only labels from filtered data
      allLabels = this.getAllLabels(data);
      // search by labels
      data = this.doSearch(data);

      if( data.length > 0 ){
        $data = <SiteList data={data} userFilter={userFilter} />
      }else{
        $data = <EmptyList text={NO_MATCH_TEXT}/>
      }
    }else{
      $data = <EmptyList text={NO_DATA_TEXT}/>
    }

    return (
      <div className="m-b-sm m-t-xl">
        <AjaxPoller onFetch={actions.fetchSites}/>
        <div className="grv-portal-apps-toolbar m-b-sm">
          <h3 className="no-margins">
            <span>Clusters </span>
            <WhatIs.OpsCenterClusters className="grv-portal-whatis-trigger"/>
          </h3>
          <div className="grv-portal-apps-toolbar-controls">
            <div className="m-r-lg">
              <label className="m-r-sm">Owner</label>
              <div className="btn-group">
                <button type="button" onClick={ () => this.onUserFilterClick(UserFilterEnum.EVERYONE) } className={ this.getUserBtnClass(UserFilterEnum.EVERYONE) }>Everyone</button>
                <button type="button" onClick={ () => this.onUserFilterClick(UserFilterEnum.ME) } className={ this.getUserBtnClass(UserFilterEnum.ME) }>Me </button>
              </div>
            </div>
            <div style={{width: '300px'}}>
              <SearchQuery labels={allLabels} onChange={this.onChangeSearchValue} />
            </div>
          </div>
        </div>
        <div className="grv-portal-list">
          {$data}
        </div>
      </div>
    )
  }
});

const SearchablePropEnum = {
  APP: 'appName',
  LABELS: 'labels',
  DOMAIN: 'domainName',
  CREATED_BY: 'createdBy'
}

const builtInLabels = {
  'cluster': SearchablePropEnum.DOMAIN,
  'createdBy': SearchablePropEnum.CREATED_BY,
  'application': SearchablePropEnum.APP
}

const doesMatch = (a='', b='') => {
  a = a.toString().toLocaleUpperCase();
  b = b.toString().toLocaleUpperCase();
  return a.indexOf(b) !== -1;
}

export default PortalSites;