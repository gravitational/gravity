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

import React from 'react'
import { storiesOf } from '@storybook/react'
import K8sResourceViewer from './K8sResourceViewer'

storiesOf('Gravity/K8s', module)
  .add('K8sResourceViewer', () => {
    const props = {
      data: data,
      title: "resource name",
      onClose(){ },
    }

    return (
      <K8sResourceViewer {...props} />
    );
  });

const data = "var period = 5m\nvar every = 1m\nvar warnRate = 75\nvar warnReset = 50\nvar critRate = 90\nvar critReset = 75\n\nvar usage_rate = stream\n    |from()\n        .measurement('cpu/usage_rate')\n        .groupBy('nodename')\n        .where(lambda: \"type\" == 'node')\n    |window()\n        .period(period)\n        .every(every)\n\nvar cpu_total = stream\n    |from()\n        .measurement('cpu/node_capacity')\n        .groupBy('nodename')\n        .where(lambda: \"type\" == 'node')\n    |window()\n        .period(period)\n        .every(every)\n\nvar percent_used = usage_rate\n    |join(cpu_total)\n        .as('usage_rate', 'total')\n        .tolerance(30s)\n        .streamName('percent_used')\n    |eval(lambda: (float(\"usage_rate.value\") * 100.0) / float(\"total.value\"))\n        .as('percent_usage')\n    |mean('percent_usage')\n        .as('avg_percent_used')\n\nvar trigger = percent_used\n    |alert()\n        .message('{{ .Level}} / Node {{ index .Tags \"nodename\" }} has high cpu usage: {{ index .Fields \"avg_percent_used\" }}%')\n        .warn(lambda: \"avg_percent_used\" > warnRate)\n        .warnReset(lambda: \"avg_percent_used\" < warnReset)\n        .crit(lambda: \"avg_percent_used\" > critRate)\n        .critReset(lambda: \"avg_percent_used\" < critReset)\n        .stateChangesOnly()\n        .details('''\n<b>{{ .Message }}</b>\n<p>Level: {{ .Level }}</p>\n<p>Nodename: {{ index .Tags \"nodename\" }}</p>\n<p>Usage: {{ index .Fields \"avg_percent_used\"  | printf \"%0.2f\" }}%</p>\n''')\n        .email()\n        .log('/var/lib/kapacitor/logs/high_cpu.log')\n        .mode(0644)\n"
