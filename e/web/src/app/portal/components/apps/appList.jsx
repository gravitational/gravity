import React from 'react';
import semver from 'semver';
import { groupBy, values } from 'lodash';
import reactor from 'oss-app/reactor';
import appsGetters from 'oss-app/flux/apps/getters';
import { PagedTable } from 'oss-app/components/common/tables/pagedTable.jsx';

import {
  Column,
  SortHeaderCell,
  SortTypes } from 'oss-app/components/common/tables/table.jsx';

import * as portalActions from './../../flux/apps/actions';
import InstallLinkDialog from './../dialogs/installLinkDialog';
import { VersionFilterEnum } from './../enums';
import { CreatedCell, ActionCell, AppCell } from './appListCells.jsx';
import { EmptyList, sortByAppName, sortByDate } from './../items.jsx';

const APP_COL_KEY = 'displayName';
const APP_COL_VER_KEY = 'version';
const CREATED_COL_KEY = 'created';
const NO_MATCH_TEXT = 'No matching applications found';
const NO_DATA_TEXT = 'You have no applications';

const AppList = React.createClass({

  mixins: [reactor.ReactMixin],

  getInitialState(){
    return { colSortDirs: { [APP_COL_KEY]: SortTypes.DESC } };
  },

  getDataBindings() {
    return {
      pkgs: appsGetters.packages
    }
  },

  onSortChange(columnKey, sortDir){
    this.state.colSortDirs = { [columnKey]: sortDir };
    this.setState(this.state);
  },

  onRemove(appId){
    portalActions.openAppConfirmDelete(appId);
  },

  doFilter(data){
    let { verFilter=VersionFilterEnum.ALL } = this.props;
    let filteredData = data;
    if(verFilter === VersionFilterEnum.LATEST){
      let grouped = groupBy(filteredData, 'displayName');
      filteredData = values(grouped).map( onlyLatestVersions );
    }

    return filteredData;
  },

  sortAndFilter(data){
    data = this.doFilter(data);

    let columnKey = Object.getOwnPropertyNames(this.state.colSortDirs)[0];
    let sortDir = this.state.colSortDirs[columnKey];

    if(columnKey === APP_COL_KEY){
      data = data.sort(sortByAppName(sortDir, APP_COL_KEY, APP_COL_VER_KEY));
    }

    if(columnKey === CREATED_COL_KEY){
      data = data.sort(sortByDate(sortDir));
    }

    return data;
  },

  render() {
    let { pkgs=[] } = this.state;

    if(pkgs.length === 0){
      return <EmptyList text={NO_DATA_TEXT}/>
    }

    let data = this.sortAndFilter(pkgs);

    if(data.length === 0){
      return <EmptyList text={NO_MATCH_TEXT}/>
    }

    return (
      <div>
        <InstallLinkDialog ref="installLinkDlg"/>
        <PagedTable tableClass="grv-portal-apps-table" data={data} pageSize={7}>
          <Column
            header={
             <SortHeaderCell
               columnKey={APP_COL_KEY}
               sortDir={this.state.colSortDirs[APP_COL_KEY]}
               onSortChange={this.onSortChange}
               title="Application"
             />
            }
            cell={<AppCell/> }
          />
          <Column
            header={
             <SortHeaderCell
               columnKey={CREATED_COL_KEY}
               sortDir={this.state.colSortDirs[CREATED_COL_KEY]}
               onSortChange={this.onSortChange}
               title="Created"
             />
            }
            cell={<CreatedCell/> }
          />
          <Column
            onRemove={this.onRemove}
            header={null}
            cell={<ActionCell/> }
          />
        </PagedTable>
      </div>
    );
  }
});

const onlyLatestVersions = ( a=[] ) =>{
  return a.reduce( (v2, v1) => semver.gte(v2.version, v1.version) ? v2 : v1 );
}

export default AppList;