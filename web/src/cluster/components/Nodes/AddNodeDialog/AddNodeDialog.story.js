/*
Copyright 2019 Gravitational, Inc.

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

import React from 'react'
import { storiesOf } from '@storybook/react'
import { AddNodeDialog } from './AddNodeDialog';

storiesOf('Gravity/Nodes/AddNodeDialog', module)
  .add('AddNodeDialog', () => (
    <AddNodeDialog {...props} />
  ))
  .add('With Instruction', () => (
    <WithCmd />
  ));

const WithCmd = () => {
  const dRef = React.useRef(null);
  React.useEffect(() => {
    dRef.current.setSelectedProfile({ value: "node"})
    dRef.current.onContinue();
  }, [])

  return <AddNodeDialog ref={dRef} {...props}/>
}

const profiles = [
  {
    "name": "node",
    "description": "Node Jadicwiw",
    "title": "Ops Center Node",
    "requirementsText": "RAM: 40.0GB, CPU: Core x 161",
  },
  {
    "name": "node3",
    "description": "Node Hifeke",
    "title": "Ops Center Node",
    "requirementsText": "RAM: 29.0GB, CPU: Core x 219",
  },
  {
    "name": "Rodoskew",
    "description": "Node Doegeco",
    "title": "Sally Griffin Node",
    "requirementsText": "RAM: 33.0GB, CPU: Core x 200",
  },
  {
    "name": "Jelpoutu",
    "description": "Node Ehtataj",
    "title": "Charlotte Dean Node",
    "requirementsText": "RAM: 16.0GB, CPU: Core x 255",
  },
  {
    "name": "Vagjedzum",
    "description": "Node Disithu",
    "title": "Minerva Green Node",
    "requirementsText": "RAM: 5.0GB, CPU: Core x 244",
  }
]

const cmd = `curl -s --tlsv1.2 -0 -k "https://demo.gravitational.io:443/t/c1c8e48d3e7fb82fa8b9a0f6f271db97de71a081fc14c81523222ec7e35b77a1/node"`;

const props = {
  profiles,
  commands: {
    gravityDownload: cmd,
    gravityJoin: {
      node: 'run!',
      node3: 'UZ',
      Rodoskew: 'RS',
      Jelpoutu: 'CC',
    }
  },
  onClose: () => {},
}