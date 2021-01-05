const webpack = require('webpack');
var baseCfg = require('./webpack.base');

var output = Object.assign({}, baseCfg.output, {
  filename: '[name].js',
  chunkFilename: '[name].js'
});

var cfg = {

  mode: 'development',

  output: output,

  devtool: false,

  resolve: baseCfg.resolve,

  optimization: {
//    ...baseCfg.optimization,
    //runtimeChunk: true
  },

  module: {
    noParse: baseCfg.noParse,
    //strictExportPresence: true,
    rules: [
      baseCfg.rules.inlineStyle,
      baseCfg.rules.svg,
      baseCfg.rules.jsx({test: true}),
    ]
  },

  plugins:  [
    baseCfg.plugins.extractAppCss,
    new webpack.DefinePlugin({ 'process.env.NODE_ENV_TYPE': JSON.stringify('test') }),
 ]
};

module.exports = cfg;
