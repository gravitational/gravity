import React from 'react';
import moment from 'moment';
import semver from 'semver';
import { SortTypes } from 'oss-app/components/common/tables/table.jsx';

const DEFAULT_EMPTY_TEXT = 'Empty';

const EmptyList = ({text=DEFAULT_EMPTY_TEXT})=> (
  <div className="text-center text-muted grv-portal-list-empty">
    <h3 className="no-margins">{text}</h3>
  </div>
)

const sortByAppName = (sortDir, appKey, versionKey) => (a, b) => {
  let dir = sortDir === SortTypes.ASC ? -1 : 1;
  if (a[appKey] === b[appKey]) {
    return semver.compare(a[versionKey], b[versionKey]) * -1;
  }

  return a[appKey].localeCompare(b[appKey]) * dir;
}

const sortByDate = sortDir => (a, b) => {
  let dir = sortDir === SortTypes.ASC ? -1 : 1;
  if(moment(a.created).isAfter(b.created) ){
    return -1 * dir;
  }

  return dir;
}

export {
  EmptyList,
  sortByAppName,
  sortByDate
};
