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
import { StepValueEnum } from './../flux/enums';

const UserHints = ({ step, licenseUserHintText, prereqUserHintText, provisionUserHintText, progressUserHintText }) => {
  let hintTexts = {};

  hintTexts[StepValueEnum.LICENSE] = licenseUserHintText;
  hintTexts[StepValueEnum.NEW_APP] = prereqUserHintText;
  hintTexts[StepValueEnum.PROVISION] = provisionUserHintText;
  hintTexts[StepValueEnum.PROGRESS] = progressUserHintText;

  let text = hintTexts[step] || 'Your custom text here';
    
  return (
    <div className="grv-installer-hints m-t-lg">
      <h3>About this step</h3>
      <div className="text-muted m-t">
        <div style={UserHints.style}
          dangerouslySetInnerHTML={{ __html: text }} />        
      </div>
    </div>
  )
}

UserHints.style = {
  whiteSpace: 'pre-line'
}

export default UserHints;
