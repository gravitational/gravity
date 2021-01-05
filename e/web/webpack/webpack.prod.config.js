const webpack = require('webpack');
const baseCfg = require('./webpack.base');

//const BundleAnalyzerPlugin = require('webpack-bundle-analyzer').BundleAnalyzerPlugin;

var cfg = {

  entry: baseCfg.entry,
  output: baseCfg.output,
  resolve: baseCfg.resolve,

  mode: 'production',

  optimization: {
    ...baseCfg.optimization,
    minimize: true
  },

  module: {
    noParse: baseCfg.noParse,
    strictExportPresence: true,
    rules: [
      baseCfg.rules.fonts,
      baseCfg.rules.svg,
      baseCfg.rules.images,
      baseCfg.rules.jsx(),
      baseCfg.rules.css(),
      baseCfg.rules.scss()
    ]
  },

  plugins:  [
    //new BundleAnalyzerPlugin(),
    new webpack.HashedModuleIdsPlugin(),
    baseCfg.plugins.createIndexHtml(),
    baseCfg.plugins.extractAppCss(),
 ]
};

module.exports = cfg;