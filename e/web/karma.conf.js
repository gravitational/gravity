const webpackCfg = require('./webpack/webpack.test.config');

module.exports = function (config) {

  config.set({

    browsers: ['Chrome'],

    frameworks: ['mocha'],

    reporters: ['mocha'],

    //files: ['./src/app/vendor.js', 'tests.webpack.js'],

    files: ['tests.webpack.js'],

    preprocessors: {
      //'./src/app/vendor.js': [ 'webpack' ],
      'tests.webpack.js': [ 'webpack' ]
    },

    webpack: webpackCfg,

    webpackServer: {
      noInfo: true
    }
  });

};
