import React from 'react';
import $ from 'jQuery';
import moment from 'moment';
import classnames from 'classnames';

// oss imports
import connect from 'oss-app/lib/connect'
import htmlUtils from 'oss-app/lib/htmlUtils';
import Button from 'oss-app/components/common/button';
import DatePicker from 'oss-app/components/common/datePicker';
import Box from 'oss-app/components/common/boxes/box';

// local imports
import getters from './../../flux/license/getters';
import { createLicense } from './../../flux/license/actions';

const DATE_FORMAT = 'mm/dd/yyyy'
const VALIDATION_WRONG_DATE_FORMAT = `Date must be in "${DATE_FORMAT}" format`;
const VALIDATION_WRONG_FUTURE_DATE = `Date must be a future date`;

class LicenseGenerator extends React.Component {

  static propTypes = {
    attempt: React.PropTypes.object.isRequired
  }

  state = {
    expirationDate: null,
    amount: null,
    makeStrict: true
  }

  componentDidMount() {
    $.validator.addMethod("grvDate", value => {
      return moment(value, 'M/D/YYYY', true).isValid();
    }, VALIDATION_WRONG_DATE_FORMAT);


    $.validator.addMethod("grvDateFuture", value => {
      return moment(value, 'M/D/YYYY', true).isAfter(Date.now());
    }, VALIDATION_WRONG_FUTURE_DATE);

    $(this.refs.addr).focus();

    $(this.refs.form).validate({
      rules: {
        amountField:{
          number: true,
          required: true
        },
        expDateField: {
          required: true,
          grvDate: true,
          grvDateFuture: true
        }
      }
    })
  }

  onCopyClick = (textToCopy, event) =>  {
    event.preventDefault();
    htmlUtils.copyToClipboard(textToCopy);
    htmlUtils.selectElementContent(this.refs.command);
  }

  onChangeExpirationDate = value => {
    this.setState({ expirationDate: value});
  }

  onChangeAmount = e => {
    this.setState({ amount: e.target.value })
  }

  onMakeStrict = ()=> {
    this.setState({ makeStrict: !this.state.makeStrict })
  }

  onClick = () => {
    if ($(this.refs.form).valid()) {
      let {amount, expirationDate, makeStrict} = this.state;
      expirationDate = new Date(expirationDate);
      expirationDate = expirationDate.toISOString();
      amount = parseInt(amount, 10);
      createLicense(amount, expirationDate, makeStrict);
    }
  }

  render() {
    const { isProcessing, isSuccess, isFailed, message } = this.props.attempt;
    const { expirationDate, makeStrict } = this.state;
    const text = isSuccess ? message : null;
    const boxClass = classnames("grv-settings-license", { "--no-stretch": !text });
    const dpSettings = {
      startDate: '+0d'
    }

    return (
      <Box title="Generate New License" className={boxClass}>
        <form ref="form">
          <div className="grv-settings-license-form">
            <div className="form-group m-r">
              <input
                autoFocus
                min="1" max="100"
                onChange={this.onChangeAmount}
                name="amountField"
                className="form-control m-r"
                placeholder="Max number of servers"/>
            </div>
            <div className="form-group" style={ {maxWidth: "150px" }} >
              <DatePicker
                settings={dpSettings}
                name="expDateField"
                placeholder={`Expiration date "${DATE_FORMAT}"`}
                value={expirationDate}
                onChange={this.onChangeExpirationDate} />
            </div>
          </div>
          <div >
            <label className="checkbox-inline">
              <input type="checkbox" checked={makeStrict} onChange={this.onMakeStrict} />
              <span>Stop the application when license expires</span>
            </label>
          </div>
          <div className="m-t">
            <Button className="btn btn-primary m-r-xs"
              size="sm"
              onClick={this.onClick}
              isPrimary={true}
              isProcessing={isProcessing}>
              Generate
            </Button>
            { isFailed && <label className="error">{message}</label> }
          </div>
        </form>
        {text &&
          <div className="grv-settings-license-new m-t p-sm text-muted">
            <div className="grv-settings-license-new-bar">
              <div className="pull-left">
                <h3>New license</h3>
              </div>
              <div className="pull-right">
                <button
                  autoFocus
                  onClick={this.onCopyClick.bind(this, text)}
                  className="btn btn-sm btn-primary pull-right">Copy License</button>
              </div>
            </div>
            <span ref="command"
              className="form-conrol m-t-sm grv-settings-license-new-license"
              defaultValue={text}>{text}</span>
          </div>
        }
      </Box>
    );
  }
}

const mapFluxToProps = () => ({
  attempt: getters.createLicenseAttempt
})

export default connect(mapFluxToProps)(LicenseGenerator)
