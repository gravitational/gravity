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
import OverlayTrigger from './common/overlayTrigger';
import * as Icons from './icons';
import Popover from './common/popover';
import cfg from 'app/config';
import { LinkEnum } from 'app/services/enums';

const makeTrigger = ({ triger="click", ...props }) => (
  <OverlayTrigger {...props} triger={triger} >  
    <Icons.Question/>  
  </OverlayTrigger>
)

const AwsSessionToken = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Amazon Session Tokens">
      <div>
        <a href={LinkEnum.AWS_SESSION_TOKEN} target="_blank"> Session Tokens </a>
        are used for getting temporary access to an AWS account. Use this option if your organization requires you to use temporary credentials.
      </div>
    </Popover>
  )
})

const AwsInstanceType = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Amazon Instance Type">
      <div>Read about 
        <a href={LinkEnum.AWS_INSTANCE_TYPES} target="_blank"> Amazon Instance Types in AWS documentation</a>
      </div>
    </Popover>
  )
})

const AwsAccessKey = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Amazon Access Key">
      <div>Read about 
        <a href={LinkEnum.AWS_ACCESS_KEY} target="_blank"> Access Keys in AWS documentation</a>
      </div>
    </Popover>
  )
})

const AwsRegion = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Amazon Regions">
      <div>Read about 
        <a href={LinkEnum.AWS_REGIONS} target="_blank"> AWS regions in the AWS documentation</a>
      </div>
    </Popover>
  )
})

const AwsPairs = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Amazon Key Pairs">
      <div>Read about
        <a href={LinkEnum.AWS_KEY_PAIRS} target="_blank"> AWS Key Pairs in the AWS documentation</a>
      </div>
    </Popover>
  )
})

const AwsVPC = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Amazon Virtual Private Clouds">
      <div>Read about 
        <a href={LinkEnum.AWS_REGIONS} target="_blank"> Amazon Virtual Private Clouds in the AWS documentation</a>
      </div>
    </Popover>
  )
})

const NewServer = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="New Servers">
      <div> Telekube will provision new servers in your AWS account </div>
    </Popover>
  )
})

const UseExistingServer = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Existing Server">
      <div> Telekube will give you a command to run on your existing servers </div>
    </Popover>
  )
})

const IpAddress = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title={cfg.getAgentDeviceIpv4().labelText}>
      <div> {cfg.getAgentDeviceIpv4().toolipText }</div>
    </Popover>
  )
})

const ServerDevice = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title={props.title}>
      <div> {props.tooltip}</div>
    </Popover>
  )
})

const OpsCenterApps = props => makeTrigger({
  ...props,    
  overlay: (
    <Popover title="Application Bundles" className="grv-whatis-application-popover">
      <div> 
        <p>This table shows all of the Application Bundles that have been published to your Ops Center. </p>        
        <span>See </span>
        <a href={LinkEnum.DOC_OPSCENTER_PACKAGE} target="_blank">Packaging and Deployment</a>
        <span> in the documentation for more information </span>        
      </div>
    </Popover>
  )
})

const OpsCenterClusters = props => makeTrigger({
  ...props,    
  overlay: (
    <Popover title="Clusters" className="grv-whatis-cluster-popover">
      <div> 
        <p>This table shows all of the Clusters that have been deployed from your Ops Center.</p>        
        <span>See </span>
        <a href={LinkEnum.DOC_OPSCENTER_CLUSTER} target="_blank">Cluster Management and Remote Management</a>
        <span> in the documentation for more information </span>        
      </div>
    </Popover>
  )
})

const Tags = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Tagging Your Cluster">
      <div>Tags enable you to optionally categorize your clusters in different ways, for example, by purpose, owner, or environment.</div>
    </Popover>
  )
})

const ServiceSubnet = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Service Subnet">
      <div>A subnet that Kubernetes uses to assign IP addresses to services.</div>
    </Popover>
  )
})

const PodSubnet = props => makeTrigger({
  ...props,
  overlay: (
    <Popover title="Pod Subnet">
      <div>A subnet that Kubernetes uses to assign IP addresses to pods.</div>
    </Popover>
  )
})

const WhatIs = {
  AwsAccessKey,
  AwsRegion,
  AwsPairs,
  AwsVPC,
  AwsInstanceType,
  AwsSessionToken,
  NewServer,
  UseExistingServer,
  IpAddress,  
  ServerDevice,
  OpsCenterApps,
  OpsCenterClusters,
  Tags,
  PodSubnet,
  ServiceSubnet
}

export default WhatIs;