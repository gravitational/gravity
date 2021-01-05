const webpack = require('webpack');
const baseCfg = require('./webpack.base');

const output = Object.assign({}, baseCfg.output, {
  filename: '[name].js',
  chunkFilename: '[name].js'
});

var cfg = {

  entry: baseCfg.entry,
  output: output,
  resolve: baseCfg.resolve,

  devtool: false,

  mode: 'development',

  optimization: baseCfg.optimization,

  module: {
    noParse: baseCfg.noParse,
    strictExportPresence: true,
    rules: [
      baseCfg.rules.fonts,
      baseCfg.rules.svg,
      baseCfg.rules.images,
      baseCfg.rules.jsx({ withHot: true}),
      baseCfg.rules.css(),
      baseCfg.rules.scss({ dev: true }),
    ]
  },

  plugins: [
    new webpack.HotModuleReplacementPlugin(),
    baseCfg.plugins.createIndexHtml(),
 ]
};

module.exports = cfg;