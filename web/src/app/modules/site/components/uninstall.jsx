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
import classnames from 'classnames';
import * as Msgs from 'app/components/msgPage.jsx';
import Indicator from 'app/components/common/indicator';
import cfg from 'app/config';
import getters from './../flux/uninstall/getters';
import { fetchSiteUninstallStatus }  from './../flux/uninstall/actions';

import { OpStateEnum } from 'app/services/enums';
import LogViewer from 'app/components/logViewer';
import connect from 'app/lib/connect';

import AjaxPoller, {SiteOperationLogProvider} from 'app/components/dataProviders';
const POLL_INTERVAL = 3000;

export const Success = ({url}) => (
  <div className="grv-site-uninstall-progress-result">
    <i style={{ 'fontSize': '100px' }} className="fa fa-check" />
    <h3 className="m-t-lg">This cluster has been successfully deleted!
    </h3>
    <a className="btn btn-primary m-t" href={url}>Navigate to OpsCenter</a>
  </div>
)

const ProgressIndicactor = props => {
  const { step = 0, isError, message } = props;
  const progress = (step + 1) * 10;
  return (
    <div className="grv-site-uninstall-progress-indicator">
      <div className="grv-site-uninstall-progress-indicator-line m-t">
        <div
          className="progress-bar progress-bar-info"
          role="progressbar"
          aria-valuenow="20"
          aria-valuemin="0"
          aria-valuemax="100"
          style={{
          "width": progress + "%"
        }}></div>
      </div>
      { !isError &&
        <small className="text-muted">
          <i className="fa m-r-xs fa-cog fa-lg fa-spin" aria-hidden="true" />
          <strong className="m-r-sm">Step {step+1} out of 10 - {message} </strong>
        </small>
      }
    </div>
    );
  }

class Progress extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      isLogsVisible: false
    }
  }

  onToggleLogs = () => {
    this.setState({
      isLogsVisible: !this.state.isLogsVisible
    }, () => this.scrollIntoView())
  }

  scrollIntoView() {
    let container = document.querySelector('.grv-site-uninstall-logviewer');
    container.scrollIntoView()
  }

  render() {
    const { siteId, opId, isError } = this.props;
    const { isLogsVisible } = this.state;
    const expandIconClass = classnames('fa', {
      'fa-caret-down': isLogsVisible,
      'fa-caret-up': !isLogsVisible
    });

    const className = classnames('grv-site-uninstall-progress', { '--error': isError });
    const logViewerClassName = classnames('grv-site-uninstall-logviewer m-b', {
      hidden: !isLogsVisible
    })

    let headerText = 'Deleting this cluster...';
    if (isError) {
      headerText = 'Failed to delete this cluster...';
    }

    return (
      <div className={className}>
        <div>
          <h2>{headerText}</h2>
          <ProgressIndicactor {...this.props}/>
        </div>
        <div className="m-t m-b text-center">
          <button onClick={this.onToggleLogs} className="btn btn-w-m btn-link">
            <small>Click here to see/hide executable logs </small> <i className={expandIconClass}/>
          </button>
        </div>
        <div>
          <LogViewer
            wrap={true}
            className={logViewerClassName}
            autoScroll={true}
            provider={<SiteOperationLogProvider siteId={siteId} opId={opId} />}
          />
        </div>
      </div>
    );
  }
}

class Uninstall extends React.Component {

  onRefresh = () => {
    return fetchSiteUninstallStatus(this.props.statusStore.siteDomain);
  }

  render() {
    const { initAttempt, statusStore, params } = this.props;

    if (initAttempt.isFailed) {
      return <Msgs.Failed message={initAttempt.message}/>
    }

    if (initAttempt.isProcessing) {
      return <Indicator enabled={true} type={'bounce'}/>
    }

    if(!cfg.isRemoteAccess(params.siteId)){
      return <Msgs.SiteUninstall/>;
    }

    let $content = null;
    if ( statusStore.state === OpStateEnum.COMPLETED ){
      $content = <Success url={cfg.routes.app }/>
    }else{
      const progressProps = {
        siteId: statusStore.siteDomain,
        step: statusStore.step,
        message: statusStore.message,
        opId: statusStore.operationId,
        isError: statusStore.state === OpStateEnum.FAILED
      }

      $content = (
        <div>
          <AjaxPoller time={POLL_INTERVAL} onFetch={this.onRefresh} />
          <Progress {...progressProps} />
        </div>
      )
    }

    return (
      <div className="grv-site grv-site-uninstall">
        <div style={{ marginLeft: "15%", marginTop: "50px", marginRight: "15%" }}>
          {$content}
        </div>
      </div>
    )
  }
}

function mapStateToProps() {
  return {
    initAttempt: getters.initAttempt,
    statusStore: getters.store
  }
}

export default connect(mapStateToProps)(Uninstall);

