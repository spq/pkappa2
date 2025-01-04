import eslint from "@eslint/js";
import tseslint from "typescript-eslint";
import eslintConfigPrettier from "eslint-config-prettier";
import pluginVue from "eslint-plugin-vue";
import vuetify from "eslint-plugin-vuetify";
import vueTsEslintConfig from "@vue/eslint-config-typescript";

export default tseslint.config(
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
  ...pluginVue.configs["flat/strongly-recommended"],
  ...vuetify.configs["flat/recommended"],
  ...vueTsEslintConfig({ extends: ["recommendedTypeChecked"] }),
  {
    // https://typescript-eslint.io/rules/
    rules: {
      "no-console": "off",
      "vue/multi-word-component-names": "off",
      "vue/no-reserved-component-names": "off",
      "vue/prefer-import-from-vue": "off",
    },
  },
  eslintConfigPrettier,
);
