module.exports = function (api) {
  api.cache(true);
  const presets = ['@babel/preset-react', ["@babel/preset-env"]];
  const plugins = [
    '@babel/plugin-proposal-class-properties',
    '@babel/plugin-proposal-object-rest-spread',
    '@babel/plugin-syntax-dynamic-import'
  ];

  return {
    env: {
      test:{
        presets,
      },
      development: {
        plugins: [
          'react-hot-loader/babel',
          ...plugins,
          'babel-plugin-styled-components'
        ]
      }
    },
    presets,
    plugins
  };
}