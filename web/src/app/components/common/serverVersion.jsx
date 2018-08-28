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

import React from "react";
import cfg from "app/config";

const ServerVersion = () => {
  const ver = cfg.getServerVersion();
  const tooltip = `ver: ${ver.version}\ncommit: ${ver.gitCommit}\ntreeState: ${
    ver.gitTreeState
  }`;
  
  return (
    <div style={{ fontSize: "10px" }} className="text-muted" title={tooltip}>
      ver: {ver.version}
    </div>
  );
};

export default ServerVersion;
