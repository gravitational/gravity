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
import $ from 'jQuery';
import { storiesOf } from '@storybook/react'
import ProviderAws from './ProviderAws';
import { makeRegion } from 'app/installer/services/installer';
import InstallerStore from 'app/installer/components/store';

storiesOf('GravityInstaller', module)
  .add('ProviderAws', () => {
    const store = new InstallerStore();
    const props = {
      ...defaultProps,
      store

    }
    return (
      <ProviderAws {...props} />
    )}
  )
  .add('ProviderAws Regions', () => {
    const store = new InstallerStore();

    store.state.aws = {
      ...store.state.aws,
      authorized: true,
      regions
    }

    const props = {
      ...defaultProps,
      store
    }
    return (
      <ProviderAws {...props} />
    )}
  );

const defaultProps = {
  onContinue: () => null,
  onVerifyKeys: () => $.Deferred().resolve(regions),
}

const regions = [
  {
    "name": "us-east-2_Lekhiva",
    "endpoint": "ec2.us-east-2.amazonaws.com_Gobbazbo",
    "vpcs": [
      {
        "vpc_id": "vpc-00125061d029381db_Fugoval",
        "cidr_block": "10.1.0.0/16_Suvromaj",
        "is_default": false,
        "state": "available_Haewnet",
        "tags": {
          "KubernetesCluster": "democluster_Rolavmar",
          "Name": "democluster_Ruhoeli",
          "Terraform": "true"
        }
      }, {
        "vpc_id": "vpc-0478be1e0b1b6c83a_Rufbatub",
        "cidr_block": "10.1.0.0/16_Meparfu",
        "is_default": false,
        "state": "available_Folhuih",
        "tags": {
          "KubernetesCluster": "testcluster_Wewido",
          "Name": "testcluster_Midawzol",
          "Terraform": "true"
        }
      }, {
        "vpc_id": "vpc-59d73e30_Burejnat",
        "cidr_block": "172.31.0.0/16_Tivena",
        "is_default": true,
        "state": "available",
        "tags": {
          "Name": "default"
        }
      }
    ],
    "key_pairs": [
      {
        "name": "lele"
      }, {
        "name": "ops"
      }, {
        "name": "sergei"
      }
    ]
  },
  {
    "name": "us-east-no-default-vpc",
    "endpoint": "ec2.us-east-2.amazonaws.com",
    "vpcs": [
       {
        "vpc_id": "vpc-59d73e30",
        "cidr_block": "172.31.0.0/16",
        "is_default": false,
        "state": "available",
        "tags": {
          "Name": "default"
        }
      }
    ],
    "key_pairs": [
      {
        "name": "lele"
      }, {
        "name": "ops"
      }, {
        "name": "sergei"
      }
    ]
  }
].map(makeRegion);
