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
import moment from 'moment';
import htmlUtils from 'app/lib/htmlUtils';

import {
  GrvDialogContent,
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from 'app/components/dialogs/dialog';

export class UserTokenLink extends React.Component {
  copy = () => {        
    htmlUtils.copyToClipboard(this.props.link);
    htmlUtils.selectElementContent(this.linkRef);    
  }

  getTtlText(created, expires){    
    return moment.duration(moment(created).diff(expires)).humanize()
  }

  render() {
    const { userToken, onClose, tokenType } = this.props;
    const { created, expires, url } = userToken;
    const ttlText = this.getTtlText(created, expires)    
    return (
      <GrvDialog title="" className="grv-dialdog-sm grv-settings-users-usertoken">                  
        <GrvDialogHeader>
          <div className="grv-settings-users-usertoken-header">
            <div className="m-t-xs m-l-xs m-r">
              <i className="fa fa-user fa-2x text-info" aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">
                New Invitation
              </h3>
            </div>
          </div>                   
        </GrvDialogHeader>
        <GrvDialogContent>
          <div>                     
            <p>
            { tokenType === 'invite' && <span> The invitation has been created. It will automatically expire in {ttlText}. </span>  }
            { tokenType === 'reset' && <span> Password reset invitation has been created. It will automatically expire in {ttlText}. </span>  }
            <br></br>
            Share the invitation URL with the user:
            </p>                           
            <strong className="grv-settings-users-usertoken-link" ref={ e => this.linkRef = e} > {url} </strong>             
          </div>    
        </GrvDialogContent>          
        <GrvDialogFooter>    
          <Button  className="btn-primary m-r-sm"                        
            onClick={this.copy}>
            Click to copy
          </Button>
          <Button  className="btn-default"            
            onClick={onClose}>
            Close
          </Button>    
        </GrvDialogFooter>
      </GrvDialog>
    
    );
  }
}
