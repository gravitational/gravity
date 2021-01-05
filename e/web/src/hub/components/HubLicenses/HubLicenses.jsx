import React from 'react';
import styled from 'styled-components'
import { withState, useAttempt } from 'shared/hooks';
import { Formik } from 'formik';
import { isDate } from 'lodash';
import * as Icons from 'shared/components/Icon';
import { Danger } from 'shared/components/Alert';
import Checkbox from 'e-app/hub/components/components/Checkbox';
import LicenseTextAre from './LicenseTextArea';
import { createLicense } from 'e-app/hub/services/license';
import htmlUtils from 'oss-app/lib/htmlUtils';
import ExpirationDate from './ExpirationDate';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from '../components/Layout';
import { Box, Flex, Card, Input, Text, LabelInput, ButtonPrimary } from 'shared/components';

export function HubLicenses(props){
  const { attempt, license, onSetLicense, attemptActions, onCreateLicense } = props;
  const [ isStrict, setStrict ] = React.useState(true);
  const licenseRef = React.useRef();

  function changeStrict(value){
    setStrict(value);
  }

  const onSubmit = (values, actions) => {
    const { amount, expiration } = values;
    onSetLicense(null);
    attemptActions.do(() => {
      return onCreateLicense({
        amount,
        expiration,
        isStrict
      })
      .done(newLicense => {
        actions.setSubmitting(false);
        onSetLicense(newLicense);
      })
    })
  }

  function onCopy(){
    event.preventDefault();
    htmlUtils.copyToClipboard(license);
    htmlUtils.selectElementContent(licenseRef.current);
  }

  const { isProcessing, isFailed, message } = attempt;

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          Licenses
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Flex flexWrap="wrap">
        <Formik
          validate={validateForm}
          onSubmit={onSubmit}
          initialValues={{
            amount: '',
            expiration: undefined
          }}>
          {
            ({ values, errors, touched, handleSubmit, handleChange, setFieldValue }) => (
              <Card flexBasis="400px" p="4" mr="4" mb="4" minWidth="200px" alignSelf="flex-start" as="form" onSubmit={handleSubmit}>
                {isFailed && <Danger mb="4">{message}</Danger>}
                <Flex>
                  <Box mr="3" flex="1">
                    {renderLabel({ errors, touched, value: 'amount', title: 'Max Number of Nodes'})}
                    <Input
                      autoComplete="off"
                      min="0"
                      value={values.amount}
                      type="number"
                      name="amount"
                      onChange={handleChange}
                    />
                  </Box>
                  <Box flex="1">
                    {renderLabel({ errors, touched, value: 'expiration', title: 'Expiration Date'})}
                    <ExpirationDate
                      value={values.expiration}
                      onChange={ value => setFieldValue('expiration', value) }
                      type="text"
                      name="expiration"
                    />
                  </Box>
                </Flex>
                <Checkbox color="text.primary" mb="3" label="Stop the application when license expires"
                  value={isStrict}
                  onChange={changeStrict}
                />
                <ButtonPrimary block type="submit" disabled={isProcessing}>
                  Generate License
                </ButtonPrimary>
              </Card>
            )}
          </Formik>
        { license && (
          <Card bg="white" color="black" p="4" as={Flex} flex="1" minWidth="800px">
            <Flex width="400px" flexDirection="column" mr="4"  justifyContent="space-between">
              <Box>
                <Flex alignItems="center" mb="3">
                  <Icons.License color="inherit"  fontSize={8} mr={2} />
                  <Text typography="h4" color="text.onLight" as="span">
                    CLUSTER LICENSE
                  </Text>
                </Flex>
                <Text typography="h5" mt={3}>INSTRUCTIONS (command line)</Text>
                <Text typography="body1" mt={3}>1. Copy the license and save it as a <StyledCode>license.pem</StyledCode> file</Text>
                <Text typography="body1" mt={2}>2. Run the following command <br/><StyledCode>gravity install --license=$(cat license.pem)</StyledCode></Text>
              </Box>
              <ButtonPrimary onClick={onCopy}>
                Click to Copy
              </ButtonPrimary>
            </Flex>
            <LicenseTextAre width="100%" ref={licenseRef} text={license} style={{maxHeight: "300px", overflow: "auto"}}/>
          </Card>
        )}
      </Flex>
    </FeatureBox>
  )
}


const StyledCode = styled.code`
  color: ${ props => props.theme.colors.info};
  font-family: ${ props => props.theme.fonts.mono};
  font-size: 12px;
`

function renderLabel({ errors, touched, title, value }){
  const hasErrors = Boolean(errors[value] && touched[value]);
  const text = hasErrors ? errors[value] : title;
  return (
    <LabelInput hasError={hasErrors}>
      {text}
    </LabelInput>
  )
}

function validateForm(values){
  const errors = {};
  if (!values.amount) {
    errors.amount = 'required field';
  }else if (values.amount <= 0) {
    errors.amount = ' should be greater than 0';
  }

  if (!values.expiration || !isDate(values.expiration)){
    errors.expiration = 'invalid date';
  }

  return errors;
}


function mapState(){
  const [ attempt, attemptActions ] = useAttempt();
  const [ license, onSetLicense ] = React.useState(null);
  return {
    onCreateLicense: createLicense,
    onSetLicense,
    attempt,
    attemptActions,
    license,
  }
}

export default withState(mapState)(HubLicenses)