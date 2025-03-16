import eslint from "@eslint/js";
import eslintConfigPrettier from "eslint-config-prettier/flat";
import pluginVue from "eslint-plugin-vue";
import globals from "globals";
import {
  defineConfigWithVueTs,
  vueTsConfigs,
} from "@vue/eslint-config-typescript";

export default defineConfigWithVueTs(
  {
    files: ["**/*.{ts,mts,tsx,vue}"],
  },
  {
    ignores: [
      "**/*.guard.ts",
      "**/dist/",
      "**/node_modules/",
      "src/parser/query.ts",
    ],
  },
  eslint.configs.recommended,
  pluginVue.configs["flat/strongly-recommended"],
  vueTsConfigs.recommendedTypeChecked,
  {
    // https://typescript-eslint.io/rules/
    rules: {
      "no-console": "off",
      "vue/multi-word-component-names": "off",
      "vue/no-reserved-component-names": "off",
      "vue/prefer-import-from-vue": "off",
    },
    languageOptions: {
      sourceType: "module",
      globals: {
        ...globals.browser,
      },
    },
  },
  eslintConfigPrettier,
);
