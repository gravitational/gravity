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
import PropTypes from 'prop-types';
import * as Alerts from 'shared/components/Alert';
import Dialog from 'shared/components/Dialog';
import { Flex, ButtonPrimary, ButtonSecondary, Text, Box } from 'shared/components';
import YamlEditor from './YamlEditor';
import Attempt from './Attempt';
import Tabs from './Tabs';

class ConfigMapEditor extends React.Component {

  state = {
    activeTabIndex: 0,
    dirtyTabs: []
  }

  onChangeTab = activeTabIndex => {
    this.setState({activeTabIndex})
  }

  makeTabDirty = (index, value) => {
    const { dirtyTabs } = this.state;
    dirtyTabs[index] = value;
    this.setState({
      dirtyTabs
    })
  }

  onSave = attemptActions => {
    const { data } = this.props.configMap;
    const newContent = this.yamlEditorRef.getContent();
    const changes = {};

    data.forEach((item, index) => {
      changes[item.name] = newContent[index]
    });

    attemptActions
      .do(() => this.props.onSave(changes))
      .then(this.props.onClose)
  }

  render() {
    const { onClose, configMap } = this.props;
    const { data = [], id, name, namespace } = configMap;
    const { activeTabIndex,  dirtyTabs } = this.state;
    const disabledSave = !dirtyTabs.some( t => t === true);

    return (
      <Dialog
        dialogCss={dialogCss}
        disableEscapeKeyDown={false}
        onClose={onClose}
        open={true}
        >
        <Attempt onRun={this.onSave}>
          {({ attempt, run }) => (
            <Flex flex="1" flexDirection="column">
              <Flex my="4" mx="5" justifyContent="space-between">
                <Text typography="h4" color="primary.contrastText">{name}</Text>
                <Text as="span">NAMESPACE: {namespace}</Text>
              </Flex>
              {attempt.isFailed &&  (
                <Alerts.Danger mx="5" mb="4">
                  {attempt.message}
                </Alerts.Danger>
              )}
              <Tabs mx="5"
                items={data}
                onSelect={this.onChangeTab}
                activeTab={activeTabIndex}
                dirtyTabs={dirtyTabs}
              />
              <Flex flex="1" mx="5">
                <YamlEditor
                  ref={ e => this.yamlEditorRef = e}
                  id={id}
                  onDirty={this.makeTabDirty}
                  initialData={data}
                  activeIndex={activeTabIndex}
              />
              </Flex>
              <Box m="5">
                <ButtonPrimary onClick={run} disabled={disabledSave || attempt.isProcessing}  mr="3">
                  Save Changes
                </ButtonPrimary>
                <ButtonSecondary disabled={attempt.isProcessing} onClick={onClose}>
                  CANCEL
                </ButtonSecondary>
              </Box>
            </Flex>
          )}
        </Attempt>
      </Dialog>
    )
  }
}

ConfigMapEditor.propTypes = {
  configMap: PropTypes.object.isRequired,
  onSave: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
}

const dialogCss = () => `
  height: 80%
  width: calc(100% - 20%);
  max-width: 1400px;
`

export default ConfigMapEditor;
