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
import {Success, Failure } from './items';
import getters from './../../flux/progress/getters';
import { fetchOpProgress} from './../../flux/progress/actions';
import LogViewer from 'app/components/logViewer';
import connect from 'app/lib/connect';
import cfg from 'app/config';

import { SiteOperationLogProvider } from 'app/components/dataProviders';

const PROGRESS_STATE_STRINGS = [
  'Provisioning Instances',
  'Connecting to instances',
  'Verifying instances',
  'Preparing configuration',
  'Installing dependencies',
  'Installing platform',
  'Installing application',
  'Verifying application',
  'Connecting to application'
];

class ProgressIndicactor extends React.Component {

  static propTypes = {
   options: React.PropTypes.array.isRequired
  }

  renderDetailsItem({text, key, isCompleted, isCurrent}){
    let iconClass = classnames('fa fa-lg', {
      'fa-check-circle': isCompleted,
      'fa-cog fa-spin': isCurrent,
      'hidden': !isCompleted && !isCurrent
    });

    let itemClass = classnames('list-group-item text-left', {
      '--current': isCurrent,
      '--completed': isCompleted
    });

    return (
      <li key={key} className={itemClass}>
        <span className="p-w">{text}</span>
        <i className={iconClass}></i>
      </li>
    )
  }

  render() {
    let {value=0, options} = this.props;
    let $bubles = [];
    let $bublesDescription = [];
    let progressLineActiveWidth = (100 / (options.length)) * value;

    for(var i = 0; i < 3; i++){
      let $bublesDescriptionItems = [];
      let bubbleClass = classnames('grv-item', {
        'grv-active': Math.floor(value / 3) > i
      });

      $bubles.push(<div key={i} className={bubbleClass}><i className="fa fa-check-circle"></i></div>);

      for(var j = i*3; j < (i*3) + 3; j++){
        let detailItem = {
          isCompleted: j < value,
          isCurrent: value === j,
          text: options[j],
          key: j
        }

        $bublesDescriptionItems.push(this.renderDetailsItem(detailItem));
      }

      let key = `${i}+${j}`;
      $bublesDescription.push(
        <div key={key} className="grv-item">
          <ul className="list-group">
            {$bublesDescriptionItems}
          </ul>
        </div>
      )
    }

    return (
      <div className="grv-installer-progres-indicator">
        <div className="grv-installer-progres-indicator-steps m-t-lg">
          {$bubles}
        </div>
        <div className="grv-installer-progres-indicator-line m-t">
          <div
            className="progress-bar progress-bar-info"
            role="progressbar"
            aria-valuenow="20"
            aria-valuemin="0"
            aria-valuemax="100"
            style={{"width": progressLineActiveWidth + "%"}}>
          </div>
        </div>
        <div className="grv-installer-progres-indicator-steps-description">
          {$bublesDescription}
        </div>
      </div>
    );
  }
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
    }, () =>   this.scrollIntoView())
  }

  scrollIntoView() {
    const container = document.querySelector('.grv-installer-logviewer');
    container.scrollIntoView()
  }

  componentDidMount(){
    fetchOpProgress();
    this.refreshInterval = setInterval(fetchOpProgress, 3000);
  }

  componentWillUnmount() {
    clearInterval(this.refreshInterval);
  }

  render() {
    if(!this.props.model){
      return null;
    }

    const { isError, isCompleted, step, siteId, opId, crashReportUrl } = this.props.model;
    const { isLogsVisible } = this.state;
    const completeInstallUrl = cfg.getInstallerLastStepUrl(siteId);
    const expandIconClass = classnames('fa', {
      'fa-caret-down': isLogsVisible,
      'fa-caret-up': !isLogsVisible
    });

    const logViewerClassName = classnames('grv-installer-logviewer m-b', {
      hidden: !isLogsVisible
    })

    return (
      <div>
        { isCompleted && <Success siteUrl={completeInstallUrl}/> }
        { isError ? <Failure tarballUrl={crashReportUrl}/> :
          <div>
            <div className="inline">
              <h3 className="inline">Installation progress</h3>
            </div>
            <ProgressIndicactor value={step} options={PROGRESS_STATE_STRINGS} />
          </div>
        }
        <div className="m-t-xl m-b-xl text-center">
          <button onClick={this.onToggleLogs} className="btn btn-w-m btn-link">
            <small>Click here to see/hide executable logs </small>
            <i className={expandIconClass} />
          </button>
        </div>
        <div>
          <LogViewer
            wrap={true}
            className={logViewerClassName}
            autoScroll={true}
            provider={ <SiteOperationLogProvider siteId={siteId} opId={opId} /> }
          />
        </div>
      </div>
    );
  }
}

function mapStateToProps() {
  return {
    model: getters.installProgress()
  }
}

export default connect(mapStateToProps)(Progress);