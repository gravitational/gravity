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
import { Link } from 'react-router';
import classnames from 'classnames';
import connect from 'app/lib/connect';
import { K8sPodDisplayStatusEnum } from 'app/services/enums'
import { isMatch } from 'app/lib/objectUtils';
import { openServerTerminal } from './../../flux/actions';
import * as userAclFlux from 'app/flux/userAcl';
import * as Dropdowns from 'app/components/common/dropDownMenu';
import {
  Table,
  Column,
  Cell,
  RowDetails,
  ToggableCell
} from 'app/components/common/tables/table.jsx';

import { K8sPodsProvider } from './dataProviders';
import podGetters from './../../flux/k8sPods/getters';
import { Wrap, JsonContent } from './items.jsx';

const NAMESPACE_KEY = 'podNamespace';
const stopPropagation = e => { e.stopPropagation(); e.preventDefault(); }

const NameCell = ({ rowIndex, expanded, monitoringEnabled, data }) => {
  let {podName, podMonitorUrl, podHostIp, podIp='', podLogUrl} = data[rowIndex];
  let isExpanded = expanded[rowIndex] === true;

  return (
    <ToggableCell isExpanded={isExpanded} className="grv-site-k8s-pods-name">
      <div>
        <Dropdowns.Menu text={podName} onClick={stopPropagation}>
            { monitoringEnabled &&
              <Dropdowns.MenuItem>
                <Link to={podMonitorUrl}>Monitoring</Link>
              </Dropdowns.MenuItem>
            }
            <Dropdowns.MenuItem>
              <Link to={podLogUrl}>Logs</Link>
            </Dropdowns.MenuItem>
        </Dropdowns.Menu>
        <div><small>host: {podHostIp}</small></div>
        { podIp.length > 0 &&  <small>pod: {podIp}</small> }
      </div>
    </ToggableCell>
  )
};

const StatusCell = ({ rowIndex, data, ...props }) => {
  const { status, statusDisplay } = data[rowIndex];
  const classname = classnames({
    'text-success': status === K8sPodDisplayStatusEnum.RUNNING,
    'text-danger': status === K8sPodDisplayStatusEnum.FAILED,
    'text-warning': status === K8sPodDisplayStatusEnum.PENDING || status === K8sPodDisplayStatusEnum.TERMINATED
  })

  return (
    <Cell {...props} className="grv-site-k8s-pods-status">
      <strong className={classname}>{statusDisplay}</strong>
    </Cell>
  )
};

const ContainerCell = ({ rowIndex, expanded, data, sshLogins }) => {
  const { podName, podNamespace } = data[rowIndex];
  const { containers } = data[rowIndex];
  const isExpanded = expanded[rowIndex];

  const makeOnClick = (containerName, login) => () =>  {
    const pod = {
      namespace: podNamespace,
      name: podName,
      container: containerName
    }

    openServerTerminal({ pod, login });
  }

  const makeLoginInputKeyPress = containerName => e =>  {
    if (e.key === 'Enter' && e.target.value) {
      const pod = {
        namespace: podNamespace,
        name: podName,
        container: containerName
      }
      openServerTerminal({ pod, login: e.target.value });
    }
  }

  const $containerItems = containers.map((item, key) => {
    const { name, logUrl } = item;
    const menuHeaterText = `SSH to ${name} as:`
    const $menuItems = [];
    for (var i = 0; i < sshLogins.size; i++){
      const login = sshLogins.get(i);
      $menuItems.push(
        <Dropdowns.MenuItemLogin key={i} onClick={makeOnClick(name, login)} text={login}/>
      );
    }

    return (
      <Dropdowns.Menu key={key} text={name} onClick={stopPropagation}>
        <Dropdowns.MenuItemTitle text={menuHeaterText}/>
        {$menuItems}
        <Dropdowns.MenuItemLoginInput onKeyPress={makeLoginInputKeyPress(name)} />
        <Dropdowns.MenuItemDivider/>
        <Dropdowns.MenuItem>
          <Link to={logUrl}>
              Logs
          </Link>
        </Dropdowns.MenuItem>
      </Dropdowns.Menu>
    )
  });

  return (
    <Wrap className="grv-site-k8s-pods-container" isExpanded={isExpanded}>
      {$containerItems}
    </Wrap>
  )
};

const LabelCell = ({ rowIndex, expanded, data }) => {
  const { labelsText } = data[rowIndex];
  const isExpanded = expanded[rowIndex];

  const $labels = labelsText.map((item, key) => (
    <div key={key}>
      <div className="label">{item}</div>
    </div>
  ))

  return (
    <Wrap className="grv-site-k8s-pods-label grv-site-k8s-table-label" isExpanded={isExpanded}>

      {$labels}

    </Wrap>
  )
};

class PodPage extends React.Component {

  constructor(props) {
    super(props);
    this.searchableComplexProps = ['containerNames', 'labelsText'];
    this.searchableProps = ['podHostIp', 'podIp', 'podName', 'phaseValue', ...this.searchableComplexProps];
    const { monitoringEnabled } = this.props;
    this.state = {
      expanded: [],
      monitoringEnabled
    };
  }

  onRowClick = index => {
    const { expanded } = this.state;
    expanded[index] = !expanded[index];
    this.setState({expanded});
  }

  searchAndFilterCb = (targetValue, searchValue, propName) => {
    if(this.searchableComplexProps.indexOf(propName)!== -1){
      return targetValue.some((item) => {
        return item.toLocaleUpperCase().indexOf(searchValue) !==-1;
      });
    }
  }

  sortAndFilter(data){
    const { namespace, searchValue='' } = this.props;
    const filtered = data
    .filter( item => item[NAMESPACE_KEY] === namespace )
    .filter( obj=> isMatch(obj, searchValue, {
        searchableProps: this.searchableProps,
        cb: this.searchAndFilterCb
      }));

    return filtered;
  }

  render() {
    const { podInfos, userAcl } = this.props;
    const { expanded, monitoringEnabled } = this.state;
    const data = this.sortAndFilter(podInfos);
    const sshLogins = userAcl.getSshLogins();
    return (
      <div className="grv-site-k8s-pods">
        <K8sPodsProvider/>
        <Table className="grv-table-with-details grv-site-k8s-table" onRowClick={this.onRowClick} rowCount={data.length} data={data}>
          <RowDetails
            content={<JsonContent colSpan={4} expanded={expanded} columnKey="podMap" />}
          />
          <Column
            header={<Cell className="--col-name">Name</Cell> }
            cell={<NameCell expanded={expanded} monitoringEnabled={monitoringEnabled}/> }
          />
          <Column
            header={<Cell className="--col-status">Status</Cell> }
            cell={<StatusCell/>}
          />
          <Column
            header={<Cell className="--col-containers">Containers</Cell> }
            cell={<ContainerCell sshLogins={sshLogins} expanded={expanded}/>}
          />
          <Column
            header={<Cell>Labels</Cell> }
            cell={<LabelCell expanded={expanded}/>}
            />
        </Table>
      </div>
    )
  }
}

const mapStateToProps = () => ({
  podInfos: podGetters.podInfoList,
  userAcl: userAclFlux.getters.userAcl
})

export default connect(mapStateToProps)(PodPage);
