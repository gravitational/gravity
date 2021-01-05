import React from 'react';
import { download } from 'oss-app/services/downloader';
import { Cell } from 'oss-app/components/common/tables/table.jsx';

const AppCell = ({ rowIndex, data, ...props }) => {
  let { displayName, version } = data[rowIndex];
  return (
    <Cell {...props}>
      <div style={{ fontSize: '14px' }}>
        {displayName}
      </div>
      <small className="text-muted">ver. {version}</small>
    </Cell>
  )
};

const CreatedCell = ({ rowIndex, data, ...props }) => {
  let { createdDisplayDate } = data[rowIndex];
  return (
    <Cell {...props}>
      <div>
        {createdDisplayDate}
      </div>
    </Cell>
  )
};

const ActionCell = ({ rowIndex, data, ...props }) => {
  let { onRemove } = props;
  let { installUrl, id, standaloneInstallerUrl } = data[rowIndex];
  return (
    <Cell {...props}>
      <div className="btn-group grv-portal-apps-actions pull-right">
        <button type="button" className="btn btn-default btn-sm dropdown-toggle" data-toggle="dropdown" aria-haspopup="trufe" aria-expanded="false">
          <span className="m-r-xs">Actions</span>
          <span className="caret" />
        </button>
        <ul className="dropdown-menu dropdown-menu-right pull-right">
          <li>
            <a href={installUrl} target="_blank">
              <i className="fa fa-rocket m-r-xs"></i>
              <span>Install</span>
            </a>
          </li>
          <li>
            <A onClick={ () => download(standaloneInstallerUrl) }>
              <i className="fa fa-rocket m-r-xs"></i>
              <span>Download</span>
            </A>
          </li>
          <li className="divider"></li>
          <li>
            <A onClick={() => onRemove(id)}>
              <i className="fa fa-trash m-r-xs"></i>
              <span> Delete...</span>
            </A>
          </li>
        </ul>
      </div>
    </Cell>
  )
};

const A = ({children, ...props}) => <a href="" {...props} onClick={A.onClick(props)} >{children}</a>

A.onClick = props => e => {
  e.preventDefault();
  props.onClick();
}

export {
  CreatedCell,
  ActionCell,
  AppCell
}
