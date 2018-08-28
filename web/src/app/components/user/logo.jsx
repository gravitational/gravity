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
import cfg from 'app/config';
import { GravitationalLogo } from './../icons.jsx';
import { ensureImageSrc } from 'app/lib/paramUtils'; 

const Logo = () => {
  
  if(!cfg.user.logo){
    return <GravitationalLogo/>
  }

  let logo = ensureImageSrc(cfg.user.logo);

  let backgroundImage = `url(${logo})`;

  let style = {    
    backgroundImage,
    backgroundSize: 'contain',
    backgroundRepeat: 'no-repeat',
    zIndex: '1',
    margin: '25px auto',    
    height: "80px",
    width: "200px",
    backgroundPosition: "center"  
  }

  return <div style={style} />
}

export default Logo;


