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
import Button from 'app/components/common/button';
import { VersionLabel, Separator, ToolBar } from './items';
import { HistoryLinkLabel } from './../items';
import { OpTypeEnum } from 'app/services/enums';

const NewVersion = props => {         
  const {    
    canUpdate,
    version,
    releaseNotes = '',
    id,      
    opId,
    siteId,
    onClick
  } = props;

  const $content = opId ? (
    <HistoryLinkLabel opType={OpTypeEnum.OPERATION_UPDATE} siteId={siteId} />
  ) : (
    <div>
      <Button isDisabled={!canUpdate} className="btn-primary btn-sm" onClick={() => onClick(id)} >
        Update to this version
      </Button>  
    </div>  
  )

  const $header = (
    <ToolBar>
      <h3 className="grv-site-app-h3">
        <i className="fa fa-info-circle fa-lg m-r" aria-hidden="true"></i>
        <span>New Version Available</span>
        <VersionLabel version={version} />
      </h3>
      {$content}        
    </ToolBar>
  );   
  
  return (
    <div className="grv-site-app-new-ver">        
      {$header}
      <div className="row">
        <div className="col-sm-12">
          <Separator/>
          <h4 className="m-b m-t">Release notes</h4>
            <div dangerouslySetInnerHTML={{ __html: releaseNotes}}/>
        </div>
      </div>
    </div>
  )
}


export default NewVersion;