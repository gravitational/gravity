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

const bearerToken = require('./bearerToken.json');
const siteAccessResp = require('app/__tests__/apiData/site.json');
const siteResp = require('app/__tests__/apiData/site.json');
const certificate = require('app/__tests__/apiData/certificate.json');
const k8sPods = require('app/__tests__/apiData/k8sPods.json');
const k8sJobs = require('app/__tests__/apiData/k8sJobs.json');
const k8sServices = require('app/__tests__/apiData/k8sServices.json');
const k8sNodes = require('app/__tests__/apiData/k8sNodes.json');
const k8sDeployments = require('app/__tests__/apiData/k8sDeployments.json');
export {
    siteResp,
    siteAccessResp,
    certificate,
    k8sPods,
    k8sJobs,
    k8sServices,
    k8sNodes,
    k8sDeployments,
    bearerToken
}