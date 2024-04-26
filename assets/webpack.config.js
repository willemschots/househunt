const path = require('path');

// Development settings below work together with the docker-compose.yml configuration
// to enable frontend development with live reloading and hot module replacement, while
// proxying requests to the backend server.
module.exports = {
  mode: 'development',
  entry: './main.js',
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: 'bundle.js',
    // serve dist directory as /static/
    publicPath: '/static/',
  },
  devServer: {
    // watch for changes to the templates directory.
    // These files are served by our backend, so we need to reload
    // the page when they change. (See HTTP_VIEW_DIR option in cmd/server).
    watchFiles: ['templates/**/*.html'],
    // Frontend dev server will run at http://localhost:3000
    port: 3000,
    // Open the browser when the server starts.
    open: true,
    // Enable hot module replacement (HMR).
    hot: true,
    // Enable gzip compression.
    compress: true,
    // Proxy all non-static requests to the backend server.
    proxy: [
      {
        context: ['**', '!/static/'],
        target: 'http://localhost:8888', // As specified in docker-compose.yml
      },
    ],
  },
  module: {
    rules: [
      {
        test: /\.css$/i,
        include: path.resolve(__dirname),
        exclude: /node_modules/,
        use: ['style-loader', 'css-loader', 'postcss-loader'],
      },
    ],
  },
};
