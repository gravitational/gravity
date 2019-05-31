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

import React from 'react';
import styled from 'styled-components';
import Chart from 'chart.js';
import { Box, Text, Flex} from 'shared/components';
import theme from 'shared/theme';

export class LineGraph extends React.Component {

  constructor(props) {
    super(props);
    this.chartBoxRef = React.createRef();
    this.chartCtrl = null;
  }

  componentDidMount() {
    const ctx = this.chartBoxRef.current.getContext('2d');
    this.chartCtrl = new Chart(ctx, config);
  }

  componentWillUnmount() {
    this.chartCtrl && this.chartCtrl.destroy();
    this.chartCtrl = null;
  }

  init(cpuData, ramData){
    this.chartCtrl.data.datasets[0].data = [ ...cpuData];
    this.chartCtrl.data.datasets[1].data = [ ...ramData];
    this.chartCtrl.update();
  }

  add(cpu, ram){
    const [ cpuDataSet, ramDataSet ] = this.chartCtrl.data.datasets;

    cpuDataSet.data.push(cpu);
    cpuDataSet.data.shift();

    ramDataSet.data.push(ram);
    ramDataSet.data.shift();

    this.chartCtrl.update();
  }

  shouldComponentUpdate(){
    return false;
  }

  render() {
    const {...rest} = this.props;
    return (
      <Container bg="primary.dark" flexDirection="column" {...rest}>
        <Header px="3" py="3" bg="primary.light" color="text.primary" alignItems="center">
          <Text typography="h5" style={{flex: "1", flexShrink: "0"}}>
            Usage Over Time
          </Text>
          <Legend color="danger" title="CPU" mr="2" />
          <Legend color="info" title="RAM" />
        </Header>
        <ChartBox>
          <CanvasContainer>
            <canvas
              ref={this.chartBoxRef}
              onClick={this.handleOnClick}/>
            </CanvasContainer>
        </ChartBox>
      </Container>
    );
  }
}

const config = {
  type: 'line',
  data: {
    labels: ["60 sec", "50 sec", "40 sec", "30 sec", "20 sec", "10 sec", "0"],
    datasets: [{
      backgroundColor: theme.colors.info,
      borderColor: theme.colors.info,
      borderWidth: 2,
      data: [],
      fill: false,
      label: 'CPU'
    }, {
      label: 'RAM',
      fill: false,
      backgroundColor: theme.colors.danger,
      borderColor: theme.colors.danger,
      data: [],
    }]
  },
  options: {
    tooltips: {
      enabled: false
    },
    layout: {
      padding: {
        left: 8,
        right: 16,
        top: 16,
        bottom: 8
      }
    },
    maintainAspectRatio: false,
    responsive: true,
    legend: {
      display: false,
    },
    scales: {
      yAxes: [{
        ticks: {
          fontColor: theme.colors.text.primary,
          min: 0,
          max: 100,
          display: true
        }
      }],
      xAxes: [{
        ticks: {
          fontColor: theme.colors.text.primary,
          display: true
        }
      }]
    }

  }
};

const Legend = ({ color, title, ...rest}) => {
  return (
    <Flex alignItems="center" {...rest}>
      <Box bg={color} width="8px" height="8px" mr="2"/>
      <Text>{title}</Text>
  </Flex>
  )
}

const ChartBox = styled.div`
  position: relative;
  flex: 1;
  min-height: 215px;
  min-width: 200px;
`

const CanvasContainer = styled.div`
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  right: 0;
`

const Header = styled(Flex)`
  border-radius: 4px 4px 0 0;
`
const Container = styled(Flex)`
  flex: 1;
  box-shadow: 0 0 32px rgba(0, 0, 0, .12), 0 8px 32px rgba(0, 0, 0, .24);
  border-radius: 4px;
  canvas {
    background-color: ${ props => props.theme.colors.primary.main};
  }
`

LineGraph.displayName = 'LineGraph';

export default LineGraph;