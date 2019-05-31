/*
Copyright 2019 Gravitational, Inc.

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

function detectPlatform() {
  const userAgent = window.navigator.userAgent;
  return {
    isWin: userAgent.indexOf('Windows') >= 0,
    isMac: userAgent.indexOf('Macintosh') >= 0,
    isLinux: userAgent.indexOf('Linux') >= 0
  }
}

export function getUrlParameter(name, path) {
  path = path || window.location.search;
  const query = path.substring(1);
  const vars = query.split('&');
  for (var i = 0; i < vars.length; i++) {
      var pair = vars[i].split('=');
      if (decodeURIComponent(pair[0]) == name) {
          return decodeURIComponent(pair[1]);
      }
  }

  return '';
}

export const platform = detectPlatform();

