// Builds the plugin as a webpack module federation container the OpenShift
// console loads at runtime. ConsoleRemotePlugin reads package.json
// (consolePlugin) and console-extensions.json from this directory.
const path = require('path');
const { ConsoleRemotePlugin } = require('@openshift-console/dynamic-plugin-sdk-webpack');

module.exports = {
  mode: 'production',
  context: path.resolve(__dirname, 'src'),
  entry: {},
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: '[name]-bundle.js',
    chunkFilename: '[name]-chunk.js',
  },
  resolve: {
    extensions: ['.ts', '.tsx', '.js'],
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        exclude: /node_modules/,
        use: [{ loader: 'ts-loader', options: { configFile: path.resolve(__dirname, 'tsconfig.json') } }],
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'css-loader'],
      },
    ],
  },
  plugins: [new ConsoleRemotePlugin()],
  devtool: 'source-map',
  optimization: {
    chunkIds: 'named',
    minimize: true,
  },
};
