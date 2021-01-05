import React from 'react';
import { PagedTable } from 'oss-app/components/common/tables/pagedTable.jsx';
import { openSiteConfirmDelete } from 'oss-app/flux/sites/actions';
import {
  Column,
  Cell,
  SortHeaderCell,
  SortTypes } from 'oss-app/components/common/tables/table.jsx';

import { sortByAppName, sortByDate } from './../items';
import { openSiteConfirmUnlink } from './../../flux/sites/actions';
import {
  StatusCell,
  DeployedByCell,
  DeploymentCell,
  ProviderCell,
  AppNameCell,
  ActionCell } from './siteListCells.jsx';

const APP_COL_KEY = 'appDisplayName';
const APP_VER_COL_KEY = 'appVersion';
const CREATED_COL_KEY = 'created';

var SiteList = React.createClass({

  getInitialState(){
    return { colSortDirs: { [CREATED_COL_KEY]: SortTypes.DESC } };
  },

  onClickUninstall(id){
    openSiteConfirmDelete(id);
  },

  onClickUnlink(id){
    openSiteConfirmUnlink(id);
  },

  onSortChange(columnKey, sortDir){
    this.state.colSortDirs = { [columnKey]: sortDir };
    this.setState(this.state);
  },

  doSort(data){
    let { colSortDirs } = this.state;
    let columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
    let sortDir = colSortDirs[columnKey];

    if( columnKey === CREATED_COL_KEY ){
      return data.sort(sortByDate(sortDir));
    }

    if( columnKey === APP_COL_KEY ){
      return data.sort(sortByAppName(sortDir, APP_COL_KEY, APP_VER_COL_KEY ));
    }

    return data;
  },

  render() {
    let { data=[] } = this.props;
    data = this.doSort(data);
    return (
      <div className="grv-portal-sites">
        <PagedTable tableClass="grv-portal-apps-table" data={data} pageSize={10}>
          <Column
            header={<Cell/>}
            cell={<StatusCell/> }
          />
          <Column
            header={<Cell>Cluster</Cell> }
            cell={<DeploymentCell/> }
          />
          <Column
            header={
              <SortHeaderCell
                onSortChange={this.onSortChange}
                sortDir={this.state.colSortDirs[CREATED_COL_KEY]}
                columnKey={CREATED_COL_KEY}
                title="Created"
              />
            }
            cell={<DeployedByCell/> }
          />
          <Column
            header={
              <SortHeaderCell
                onSortChange={this.onSortChange}
                columnKey={APP_COL_KEY}
                sortDir={this.state.colSortDirs[APP_COL_KEY]}
                title="Application"
              />
            }
            cell={<AppNameCell/> }
          />
          <Column
            header={<Cell>Location</Cell> }
            cell={<ProviderCell/> }
          />
          <Column
            header={ null }
            cell={
              <ActionCell
                onClickUninstall={this.onClickUninstall}
                onClickUnlink={this.onClickUnlink}
              />
            }
          />
        </PagedTable>
      </div>
    );
  }
});

export default SiteList;
