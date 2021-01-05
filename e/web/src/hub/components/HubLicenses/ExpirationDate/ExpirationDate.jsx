/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';
import DayPickerInput from 'react-day-picker/DayPickerInput';
import MomentLocaleUtils, { formatDate, parseDate } from 'react-day-picker/moment';
import 'react-day-picker/lib/style.css';
import cfg from 'oss-app/config';
import { Input } from 'shared/components';

export default class ExpirationDate extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      selectedDay: undefined,
      isEmpty: true,
      isDisabled: false,
    };
  }

  handleDayChange = (selectedDay, modifiers, dayPickerInput) => {
    this.props.onChange(selectedDay);
    const input = dayPickerInput.getInput();
    this.setState({
      selectedDay,
      isEmpty: !input.value.trim(),
      isDisabled: modifiers.disabled === true,
    });
  }

  render() {
    const { selectedDay } = this.state;
    return (
      <StyledDateRange>
        <DayPickerInput
          formatDate={formatDate}
          parseDate={parseDate}
          component={Input}
          placeholder={cfg.dateFormat}
          format={cfg.dateFormat}
          value={selectedDay}
          onDayChange={this.handleDayChange}
          inputProps={{
            autoComplete: "off",
            mb: "0"
          }}
          dayPickerProps={{
            localeUtils: MomentLocaleUtils,
            disabledDays: {
              before: new Date(),
            }
          }}
        />
      </StyledDateRange>
    );
  }
}

const StyledDateRange = styled.div`
  .DayPickerInput-Overlay{
    background-color: transparent;
  }

  .DayPickerInput{
    width: 100%;
  }

  .DayPicker {
    line-height: initial;
    color: black;
    background-color: white;
    box-shadow: inset 0 2px 4px rgba(0,0,0,.24);
    box-sizing: border-box;
    border-radius: 5px;
    padding: 14px;
  }

  .DayPicker-Day {
    border-radius: 0 !important;
  }
`