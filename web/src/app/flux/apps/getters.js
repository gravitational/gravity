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

import reactor from 'app/reactor';
import semver from 'semver';
import { displayDate } from 'app/lib/dateUtils';
import { ensureImageSrc } from 'app/lib/paramUtils';

const findNewVersion = ({name, repository, version}) => {
  let appsMap = reactor.evaluate(['apps']);
  return appsMap.valueSeq()
    .filter(itemMap=>{
      let [itemRepo, itemName, itemVersion] = itemMap.get('id').split('/');
      let isTheSameApp = repository === itemRepo && name === itemName;
      return isTheSameApp && semver.gt(itemVersion, version);
    })
    .sortBy(
      mapItem => mapItem.getIn(['package', 'version']),
      semver.compare);
}

const createLogoUri = (appMap) => {
  let imageSrc = appMap.getIn(['manifest', 'logo']);  
  if(imageSrc){
    imageSrc = ensureImageSrc(imageSrc); 
  }
    
  return imageSrc;
}

const packages = [['apps'], appsMap => {
    return appsMap.valueSeq().map(itemMap=>{
      let id = itemMap.get('id');
      let pkg = itemMap.get('package').toJS();
      let { name, version, repository } = pkg;
      let displayName = itemMap.getIn(['manifest', 'metadata', 'displayName']);  
      let created = itemMap.getIn(['envelope', 'created']);
      let createdBy = itemMap.getIn(['envelope', 'created_by']);
      let createdDisplayDate = 'unknown';

      displayName = displayName || name;
      createdBy = createdBy || 'unknown';

      if(created){
        createdDisplayDate = displayDate(new Date(created));
      }

      return {
        created,
        createdDisplayDate,
        createdBy,
        installUrl: itemMap.get('installUrl'),
        pkgFileUrl: itemMap.get('pkgFileUrl'),
        standaloneInstallerUrl: itemMap.get('standaloneInstallerUrl'),
        id,
        name,
        displayName,
        version,
        repository
      }
    }).toJS();
}];

export default {
  packages,
  findNewVersion,
  createLogoUri
}
