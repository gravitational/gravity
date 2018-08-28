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

import Term from 'xterm/dist/xterm';
import Tty from './tty';
import TtyEvents from './ttyEvents';
import {debounce, isNumber} from 'lodash';
import Logger from 'app/lib/logger';
import $ from 'jQuery';

const logger = Logger.create('lib/term/terminal');
const DISCONNECT_TXT = 'disconnected';
const GRV_CLASS = 'grv-terminal';
const WINDOW_RESIZE_DEBOUNCE_DELAY = 200;

/**
 * TtyTerminal is a wrapper on top of xtermjs that handles connections
 * and resize events
 */
class TtyTerminal {

  constructor(options){
    const { addressResolver, el, scrollBack = 1000 } = options;
    this._el = el;
    this.tty = new Tty(addressResolver);
    this.ttyEvents = new TtyEvents(addressResolver);
    this.scrollBack = scrollBack
    this.rows = undefined;
    this.cols = undefined;
    this.term = null;
    this.debouncedResize = debounce(
      this._requestResize.bind(this),
      WINDOW_RESIZE_DEBOUNCE_DELAY
    );
  }

  open() {
    $(this._el).addClass(GRV_CLASS);

    // render xtermjs with default values
    this.term = new Term({
      cols: 15,
      rows: 5,
      scrollback: this.scrollBack,
      cursorBlink: false
    });

    this.term.open(this._el);

    // fit xterm to available space
    this.resize(this.cols, this.rows);

    // subscribe to xtermjs output
    this.term.on('data', data => {
      //debugger
      this.tty.send(data)
    })

    // subscribe to window resize events
    window.addEventListener('resize', this.debouncedResize);

    // subscribe to tty
    this.tty.on('reset', this.reset.bind(this));
    this.tty.on('close', this._processClose.bind(this));
    this.tty.on('data', this._processData.bind(this));

    // subscribe tty resize event (used by session player)
    this.tty.on('resize', ({h, w}) => this.resize(w, h));
    // subscribe to session resize events (triggered by other participants)
    this.ttyEvents.on('resize', ({h, w}) => this.resize(w, h));

    this.connect();
  }

  connect(){
    this.tty.connect(this.cols, this.rows);
    this.ttyEvents.connect();
  }

  destroy() {
    window.removeEventListener('resize', this.debouncedResize);
    this._disconnect();
    if(this.term !== null){
      this.term.destroy();
      this.term.removeAllListeners();
    }

    $(this._el).empty().removeClass(GRV_CLASS);
  }

  reset() {
    this.term.reset()
  }

  resize(cols, rows) {
    try {
      // if not defined, use the size of the container
      if(!isNumber(cols) || !isNumber(rows)){
        const dim = this._getDimensions();
        cols = dim.cols;
        rows = dim.rows;
      }

      if(cols === this.cols && rows === this.rows){
        return;
      }

      this.cols = cols;
      this.rows = rows;
      this.term.resize(cols, rows);
    } catch (err) {
      logger.error('xterm.resize', { w: cols, h: rows }, err);
      this.term.reset();
    }
  }

  _processData(data){
    try {
      this.term.write(data);
    } catch (err) {
      logger.error('xterm.write', data, err);
      // recover xtermjs by resetting it
      this.term.reset();
    }
  }

  _processClose(e) {
    const { reason } = e;
    let displayText = DISCONNECT_TXT;
    if (reason) {
      displayText = `${displayText}: ${reason}`;
    }

    displayText = `\x1b[31m${displayText}\x1b[m\r\n`;
    this.term.write(displayText)
  }

  _disconnect() {
    this.tty.disconnect();
    this.tty.removeAllListeners();
    this.ttyEvents.disconnect();
    this.ttyEvents.removeAllListeners();
  }

  _requestResize(){
    const { cols, rows } = this._getDimensions();
    // ensure min size
    const w = cols < 5 ? 5 : cols;
    const h = rows < 5 ? 5 : rows;

    this.resize(w, h);
    this.tty.requestResize(w, h);
  }

  _getDimensions(){
    const $container = $(this._el);
    const fakeRow = $('<div><span>&nbsp;</span></div>');

    // calculate font size using temporary div element
    $container.find('.terminal').append(fakeRow);
    const fakeColHeight = fakeRow[0].getBoundingClientRect().height;
    const fakeColWidth = fakeRow.children().first()[0].getBoundingClientRect().width;
    const width = $container[0].clientWidth;
    const height = $container[0].clientHeight;
    const cols = Math.floor(width / (fakeColWidth));
    const rows = Math.floor(height / (fakeColHeight));

    fakeRow.remove();
    return { cols, rows };
  }
}

export default TtyTerminal;