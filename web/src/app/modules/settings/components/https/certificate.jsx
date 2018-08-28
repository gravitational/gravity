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
import classnames from 'classnames';
import $ from 'jQuery';
import Layout from 'app/components/common/layout';
import Box from 'app/components/common/boxes/box';
import Button from 'app/components/common/button';
import Form from 'app/components/common/form';
import { saveTlsCert } from '../../flux/tls/actions';

const Label = ({ text, width = "200px", className = "text-bold m-t-xs", desc = null }) => ( 
  <span>
    <label style={{ width }} className={className}> {text} </label>    
    { desc  && <div className="text-muted m-t-n-sm"><small>{desc} </small></div> }
  </span>
)

class NewCert extends React.Component {

  static propTypes = {
    siteId: React.PropTypes.string.isRequired,    
    onClose: React.PropTypes.func.isRequired
  }
    
  constructor(props) {
    super(props);
    this.refForm = null;
    this.state = {
      isProcessing: false,
      certFile: null,
      privateKeyFile: null,
      intermCertFile: null
    };    
  }

  isProcessing = (isProcessing) => {
    this.setState({
      isProcessing
    })
  }

  componentDidMount() {
    $(this.refForm).validate();
  }
    
  onUpdateFiles = () => {    
    if ($(this.refForm).valid()) {      
      this.isProcessing(true);
      saveTlsCert(
        this.props.siteId,
        this.state.certFile,
        this.state.privateKeyFile,
        this.state.intermCertFile)
        .done(this.props.onClose)
        .fail(() => this.isProcessing(false));
    }
  }

  onCertFileSelected = e => {
    this.setState({
      certFile: e.target.files[0]
    });
  }

  onKeyFileSelected = e => {
    this.setState({
      privateKeyFile: e.target.files[0]
    });
  }

  onIntermFileSelected = e => {
    this.setState({
      intermCertFile: e.target.files[0]
    });
  }

  onSelectCert = () => {
    let el = document.querySelector('.grv-settings-cert-certfile');
    el.value = null;
    el.click();
  }

  onSelectPrivateKey = () => {
    let el = document.querySelector('.grv-settings-cert-key');
    el.value = null;
    el.click();
  }

  onSelectIntermCert = () => {
    let el = document.querySelector('.grv-settings-cert-intcert');
    el.value = null;
    el.click();
  }

  render() {
    let {
      certFile,
      isProcessing,
      intermCertFile,
      privateKeyFile } = this.state;        
    
    return (      
      <Box title="HTTPS Certificate" className="--no-stretch">
        <Form refCb={e => this.refForm = e} className="m-t m-b-md">
          <Layout.Flex dir="row">
            <Label text="Private Key:" desc="must be in PEM format"/>
            <FileInput name="grv-validation-name-key"
              fileName={privateKeyFile && privateKeyFile.name}
              onClick={this.onSelectPrivateKey} />
          </Layout.Flex>
          <Layout.Flex dir="row" className="m-t">
            <Label text="Certificate:" desc="must be in PEM format"/>
            <FileInput name="grv-validation-name-cert"
              fileName={certFile && certFile.name}
              onClick={this.onSelectCert} />
          </Layout.Flex>
          <Layout.Flex dir="row" className="m-t">
            <Label text="Intermediate Certificate:" desc="optional"/>
            <FileInput isRequired={false}
              fileName={intermCertFile && intermCertFile.name}
              onClick={this.onSelectIntermCert} />
          </Layout.Flex>
        </Form>
        <Button size="sm" className="btn-primary m-r-sm" onClick={this.onUpdateFiles} isProcessing={isProcessing}>
          Save
        </Button>
        <Button size="sm" className="btn-default" onClick={this.props.onClose} isDisabled={isProcessing}>
          Cancel
        </Button>
        <div className="hidden">
          <HiddenInput className="grv-settings-cert-certfile" onSelected={this.onCertFileSelected} />
          <HiddenInput className="grv-settings-cert-key" onSelected={this.onKeyFileSelected} />
          <HiddenInput className="grv-settings-cert-intcert" onSelected={this.onIntermFileSelected} />
        </div>
      </Box>              
    );
  }
}

const FileInput = ({ fileName, name, onClick, isRequired=true }) => (  
  <Layout.Flex dir="row" style={{ flex: 1 }}>
    <div style={{ width: "100%" }}>
      <input                                        
        readOnly
        name={name}
        required={isRequired}
        value={fileName || ''}
        className={classnames("form-control input-sm", { "required" : isRequired })} />                               
    </div>  
    <a type="submit" className="btn m-l-sm btn-sm btn-default" onClick={onClick}>Browse...</a>      
  </Layout.Flex>
)

const HiddenInput = ({className, onSelected}) => (
  <input type="file" accept="*.*"
    className={className}
    name="file"    
    onChange={onSelected}
    />
)

export class Certificate extends React.Component {
  constructor(props) {    
    super(props);    
    this.state = {
      isEditing: false
    };
  }

  onStartEditing = () => {
    this.setState({
     isEditing: true
   }) 
  }

  onStopEditing = () => {
    this.setState({
      isEditing: false
    })
  }

  render() {
    let { store, siteId } = this.props;
    if (this.state.isEditing) {
      return (
        <NewCert
          siteId={siteId}          
          onClose={this.onStopEditing}        
        />
      )
    }
        
    return (      
      <Box className="--no-stretch">
        <Box.Header>
          <h3>HTTPS Certificate</h3>
          <div className="text-right" style={TtlCertHeaderStyle}>                                      
            <Button size="sm" className="btn-default m-t-n-xs" onClick={this.onStartEditing}>
              <i className="fa fa-pencil m-r-xs"/>
              Change
            </Button>
          </div>
        </Box.Header>   
        <div>
          <h4 className="m-t">              
            Issued To
          </h4>
          <CertAttr name="Common Name (CN)" value={ <strong>{store.getToCn()}</strong> } />
          <CertAttr name="Organization (O)" value={store.getToOrg()} />
          <CertAttr name="Organization Unit (OU)" value={store.getToOrgUnit()} />                                                                        
          <h4 className="m-t-md">              
            Issued By
          </h4>                        
          <CertAttr name="Organization (O)" value={store.getByOrg()} />
          <CertAttr name="Organization Unit (OU)" value={store.getByOrgUnit()} />                        
          <h4 className="m-t-md">              
            Validity Period
          </h4>
          <CertAttr name="Issued On" value={store.getStartDate()} />
          <CertAttr name="Expires On" value={<strong>{store.getEndDate()}</strong>} />                    
        </div>          
      </Box>              
    )
  }
}

const CertAttr = ({ name, value }) => (
  <div className="m-l">
    <span style={{ width: "180px", display: "inline-block" }}>{name}</span>
    <span>{value}</span>
  </div>
)

const TtlCertHeaderStyle = {
  flex: "1",
  height: "20px",
  marginTop: "-3px"
}