import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import vuetify from "vite-plugin-vuetify";
import { nodePolyfills } from "vite-plugin-node-polyfills";
import { fileURLToPath } from "url";
import path from "path";
import checker from "vite-plugin-checker";

// https://vitejs.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      // vue: "@vue/compat",
      "@": path.resolve(path.dirname(fileURLToPath(import.meta.url)), "./src"),
    },
  },
  plugins: [
    vue({
      // template: {
      //   compilerOptions: {
      //     compatConfig: {
      //       MODE: 3,
      //     },
      //   },
      // },
    }),
    vuetify({
      autoImport: true,
    }),
    nodePolyfills({
      include: ["events"], // tiny-typed-emitter
    }),
    checker({
      // typescript: true,
      eslint: {
        lintCommand: "eslint --ext .js,.ts,.vue .",
      },
    }),
  ],
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
