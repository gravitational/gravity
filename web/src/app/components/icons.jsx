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

import gravitySvg from 'assets/img/grv-logo.svg';

export const GravitationalLogo = () => (
  <svg className="grv-icon-gravitational-logo"><use xlinkHref={"#"+gravitySvg.id}/></svg>
)

export const CustomerLogo = ({className, imageUri, style}) => {
  let imageStyle = {
    maxWidth: '100%',
    maxHeight: '100%',
    margin: '0 auto',
    display: 'block'
  };

  let containterStyle={
    height: '100%',
    ...style
  }

  return (
    <div style={containterStyle} className={className}>
      <img src={imageUri} style={imageStyle} />
    </div>
  )
}

export const Logs = ({title="Logs"}) => (
  <i title={title} className="fa fa-book " />
)

export const Monitoring = ({title="Monitoring"}) => (
  <i title={title} className="fa fa-area-chart" />
)

export const Delete = ({title="Delete"}) => (
  <i title={title} className="fa fa-trash" />
)

export const Question = ({title=""}) => (
  <i title={title} className="fa fa-question-circle" />
)