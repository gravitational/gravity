import React from 'react';
import DeleteSiteDialog from 'oss-app/components/sites/deleteSiteDialog';
import { deleteSite } from './../../flux/sites/actions';

const UninstallSiteDialog = () => (
  <DeleteSiteDialog onOk={deleteSite}/>
)

export default UninstallSiteDialog;
