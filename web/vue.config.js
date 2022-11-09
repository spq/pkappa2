module.exports = {
  transpileDependencies: [
    'vuetify'
  ],
  devServer: {
    allowedHosts: "all",
    proxy: {
      "^/api": {
        target: "http://localhost:8081",
        changeOrigin: true,
      },
    },
  },
}
