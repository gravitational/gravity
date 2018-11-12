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
import { ErrorPageEnum } from 'app/services/enums';

export const MSG_INFO_LOGIN_SUCCESS = 'Login was successful, you can close this window and continue using tsh.';
export const MSG_ERROR_LOGIN_FAILED = 'Login unsuccessful. Please try again.';
export const MSG_ERROR_DEFAULT = 'Internal Error';
export const MSG_ERROR_NOT_FOUND = '404 Not Found';
export const MSG_ERROR_NOT_FOUND_DETAILS = `Looks like the page you are looking for isn't here any longer.`;
export const MSG_ERROR_LINK_EXPIRED = 'This link has expired.';
export const MSG_ERROR_INVALID_USER = 'Invalid user.';
export const MSG_ERROR_INVALID_USER_DETAILS = `User is already registered for another account.`;
export const MSG_ERROR_ACCESS_DENIED = 'Access denied';

const InfoTypes = {
  LOGIN_SUCCESS: 'login_success'
};

const ErrorPage = ({ params, location }) => {
  const { type } = params;
  const { details } = location.query;
  switch (type) {
    case ErrorPageEnum.FAILED_TO_LOGIN:
      return <LoginFailed message={details} />
    case ErrorPageEnum.EXPIRED_LINK:
      return <ExpiredLink />
    case ErrorPageEnum.NOT_FOUND:
      return <NotFound />
    case ErrorPageEnum.INVALID_USER:
      return <InvalidUser />

    default:
      return <Failed message={details}/>
  }
}

const InfoPage = ({ params }) => {
  let { type } = params;

  if (type === InfoTypes.LOGIN_SUCCESS) {
    return <SuccessfulLogin/>
  }

  return <Info/>
}

const Box = props => (
  <div className="grv-msg-page">
    <div className="grv-header">
      <i className={props.iconClass}></i>
    </div>
    {props.children}
  </div>
)

const Error = props => (
  <Box iconClass="fa fa-exclamation-triangle" {...props} />
)

const Info = props => (
  <Box iconClass="fa fa-smile-o" {...props} />
)

const SuccessfulLogin = () => (
  <Info>
    <h1>{MSG_INFO_LOGIN_SUCCESS}</h1>
  </Info>
)

const NotFound = () => (
  <Error>
    <h1>{MSG_ERROR_NOT_FOUND}</h1>
    <ErrorDetails message={MSG_ERROR_NOT_FOUND_DETAILS}/>
  </Error>
)

const ErrorDetails = ({ message }) => (
   <div className="m-t text-muted" style={{ wordBreak: "break-all" }}>
    <small>{message}</small>
  </div>
)

const Failed = ({message}) => (
  <Error>
    <h1>{MSG_ERROR_DEFAULT}</h1>
    <ErrorDetails message={message}/>
  </Error>
)

const AccessDenied = ({message}) => (
  <Error>
    <h1>{MSG_ERROR_ACCESS_DENIED}</h1>
    <ErrorDetails message={message}/>
  </Error>
)

const LoginFailed = ({ message }) => (
  <Error>
    <h1>{MSG_ERROR_LOGIN_FAILED}</h1>
    <ErrorDetails message={message}/>
  </Error>
)

const InvalidUser = () => (
  <Error>
    <h1>{MSG_ERROR_INVALID_USER}</h1>
    <ErrorDetails message={MSG_ERROR_INVALID_USER_DETAILS}/>
  </Error>
)

const ExpiredLink = () => (
  <Error>
    <h1>{MSG_ERROR_LINK_EXPIRED}</h1>
  </Error>
)

const SiteOffline = ({siteId}) => (
  <Box iconClass="fa fa-plug">
    <h3>This cluster is not available from Gravity.</h3>
    <h5>To access <strong>{siteId}</strong> please use its local endpoint.</h5>
  </Box>
)

const SiteUninstall = () => (
  <Box iconClass="fa fa-trash">
    <h3>This cluster has been scheduled for deletion.</h3>
  </Box>
)

export {
  InfoPage,
  ErrorPage,
  NotFound,
  SiteOffline,
  SiteUninstall,
  Failed,
  ExpiredLink,
  AccessDenied
};
