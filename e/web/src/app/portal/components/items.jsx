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
