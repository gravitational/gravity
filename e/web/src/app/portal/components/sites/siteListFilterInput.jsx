import React from 'react';
import { keyBy } from 'lodash';
import Input from 'oss-app/components/common/input.jsx';

const TOKEN_REGX = "(([\\w-.]+):([\\w-.]+)?)+";

var SearchQuery = React.createClass({

  getInitialState(){
    return {
      query: ''
    }
  },

  shouldComponentUpdate(newProps, newState){
    let newLabels = newProps.labels;
    let oldLabels = this.props.labels;

    if(newState.query !== this.state.query){
      return true
    }else if(newLabels.length === oldLabels.length){
      return newLabels.some( (a, index) => oldLabels[index] !== a);
    }

    return true;
  },

  onClick(labelName){
    let { query } = this.state;

    query = query += ` ${labelName}:`;
    query = query.trim();

    this.refs.queryInput.focus();
    this.setState({query});
    this.onChange(query);
  },

  parseQuery(query=''){
    let { labels } = this.props;
    let info = {
      text: '',
      labels: []
    };

    let processedText = query;
    let labelMap = keyBy(labels);
    let tmp = {};
    let tokenRegex = new RegExp(TOKEN_REGX, 'g')
    let match;

    while ((match = tokenRegex.exec(query)) !== null) {
      let matchedSubStr = match[0];
      let [filterType, filterValue] = matchedSubStr.split(':');

      if(labelMap.hasOwnProperty(filterType)){
        tmp[filterType] = filterValue;
        processedText = processedText.replace(
          new RegExp(`\\b${matchedSubStr}`, 'g'), '');
      }
    }

    info.labels = tmp;
    info.text = processedText.replace(/\s\s+/g, '').trim();

    return info;
  },

  onQueryChange(event){
    let query = event.target.value;
    this.onChange(query);
  },

  onChange(query){
    this.setState({query});
    if(this.props.onChange){
      let info = this.parseQuery(query);
      this.props.onChange(info);
    }
  },

  render() {
    let { query } = this.state;
    let { labels=[] } = this.props;
    let $dropdown = this.renderFilterDropdown(labels);
    return (
      <div className="input-group input-group-sm full-width">
        {$dropdown}
        <Input
          placeholder="Search..."
          ref="queryInput"
          value={query}
          onChange={this.onChange} type="text" className="form-control"/>
      </div>
    )
  },

  renderFilterDropdown(labels) {
    if(labels.length === 0){
      return null;
    }

    let $labels = labels.map( (text, key) => (
      <li key={key}>
        <a title={"label:" + text } onClick={() => this.onClick(text)}>
          <i style={{fontSize: '10px'}} className="fa fa-tags m-r-xs" aria-hidden="true"></i>
          {text}
        </a>
      </li>
    ));

    return (
      <div className="input-group-btn">
        <button data-toggle="dropdown" className="btn btn-white dropdown-toggle" type="button">
          <span className="m-r-xs">Filter</span>
           <span className="caret"/>
        </button>
        <ul className="dropdown-menu">
          {$labels}
        </ul>
      </div>
    )
  }
});

export default SearchQuery;
