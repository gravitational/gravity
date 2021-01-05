import React from 'react';
import history from 'oss-app/services/history';
import ReactDOM from 'react-dom';
import Root from  './app/index';
import cfg from './app/config';

cfg.init(window.GRV_CONFIG);
history.init();

ReactDOM.render( ( <Root/> ), document.getElementById('app'));
