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
import { Cell, RowContent } from 'app/components/common/tables/table.jsx';

const MAX_LABELS = 3;

export const Wrap = props => {
  let { isExpanded, children=[], maxKids=MAX_LABELS } = props;
  let $more = null;
  let rowSpan = 1;
  let className = '';

  if(children.length > maxKids){
    if(isExpanded){
      className='--with-row-span'
    }else{
      $more = <div className="m-t-xs">and more...</div>
      let count = isExpanded ? children.length : Math.min(maxKids, children.length);
      children = children.slice(0, count);
    }
  }

  return (
    <Cell className={className} rowSpan={rowSpan} {...props}>
      {children}
      {$more}
    </Cell>
  )
}

export const JsonContent = ({ expanded, colSpan, rowIndex, data, columnKey }) => {  
   if(expanded[rowIndex] === true){
     let obj = data[rowIndex][columnKey];
     let json = obj.toJSON();
     json = JSON.stringify(json, null, 2);
     return (
       <RowContent className="grv-table-row-details">
         <Cell colSpan={colSpan}>
           <pre className="m-l-md"><code>{json}</code></pre>
         </Cell>
       </RowContent>
     )
   }

   return null;
 };