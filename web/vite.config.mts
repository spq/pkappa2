import vue from "@vitejs/plugin-vue";
import { nodePolyfills } from "vite-plugin-node-polyfills";
import vuetify, { transformAssetUrls } from "vite-plugin-vuetify";
import checker from "vite-plugin-checker";
import Fonts from "unplugin-fonts/vite";
import Components from "unplugin-vue-components/vite";

import { defineConfig } from "vite";
import { fileURLToPath, URL } from "node:url";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue({
      template: { transformAssetUrls },
    }),
    // https://github.com/vuetifyjs/vuetify-loader/tree/master/packages/vite-plugin#readme
    vuetify({
      autoImport: true,
      styles: {
        configFile: "src/styles/settings.scss",
      },
    }),
    Components({
      dts: "src/components.d.ts",
    }),
    nodePolyfills({
      include: ["events"], // tiny-typed-emitter
    }),
    checker({
      typescript: true,
      vueTsc: true,
      eslint: {
        lintCommand: "eslint",
        useFlatConfig: true,
      },
    }),
    Fonts({
      google: {
        families: [
          {
            name: "Roboto",
            styles: "wght@100;300;400;500;700;900",
          },
        ],
      },
    }),
  ],
  define: { "process.env": {} },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 8080,
    proxy: {
      "/api": {
        target: "http://localhost:8081",
        changeOrigin: true,
      },
      "/ws": {
        target: "ws://localhost:8081",
        ws: true,
        changeOrigin: false,
      },
    },
  },
});
