import globals from "globals";
import eslint from "@eslint/js";
import tseslint from "typescript-eslint";
import eslintConfigPrettier from "eslint-config-prettier";
import pluginVue from "eslint-plugin-vue";
import vuetify from 'eslint-plugin-vuetify'
import vueParser from "vue-eslint-parser";
import vueTsEslintConfig from "@vue/eslint-config-typescript";

export default tseslint.config(
  {
    ignores: [
      "**/*.guard.ts",
      "**/dist/",
      "**/node_modules/",
      "src/parser/query.ts",
    ],
  },
  {
    extends: [
      eslint.configs.recommended,
      ...pluginVue.configs["flat/strongly-recommended"],
      ...vuetify.configs["flat/base"],
      ...vueTsEslintConfig(),
      ...tseslint.configs.recommendedTypeChecked,
    ],
    languageOptions: {
      parser: vueParser,
      globals: globals.browser,
      parserOptions: {
        ecmaVersion: 2021,
        extraFileExtensions: [".vue"],
        sourceType: "module",
        parser: tseslint.parser,
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    files: ["**/*.{ts,vue}"],
    rules: {
      "no-console": "off",
      "no-debugger": process.env.NODE_ENV === "production" ? "warn" : "off",
      "vue/multi-word-component-names": "off",
      "vue/no-reserved-component-names": "off",
      "vue/prefer-import-from-vue": "off",
    },
  },
  eslintConfigPrettier,
);
