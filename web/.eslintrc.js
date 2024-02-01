module.exports = {
  root: true,
  env: {
    node: true,
    "vue/setup-compiler-macros": true,
  },
  parserOptions: {
    ecmaVersion: 2021,
    project: "./tsconfig.json",
    parser: "@typescript-eslint/parser",
  },
  ignorePatterns: ["**/*.js", "**/*.vue"],
  plugins: ["@typescript-eslint", "@typescript-eslint/eslint-plugin"],
  parser: "vue-eslint-parser",
  extends: [
    // add more generic rulesets here, such as:
    "eslint:recommended",
    // 'plugin:vue/vue3-recommended',
    "plugin:vue/recommended", // Use this if you are using Vue.js 2.x.
    "prettier",
    "@vue/typescript",
    "plugin:@typescript-eslint/recommended-requiring-type-checking",
    "plugin:@typescript-eslint/eslint-recommended",
    "plugin:@typescript-eslint/recommended",
    "plugin:vue/base",
  ],
  rules: {
    // override/add rules settings here, such as:
    // 'vue/no-unused-vars': 'error'
    "no-console": "off",
    "no-debugger": process.env.NODE_ENV === "production" ? "warn" : "off",
    // "@typescript-eslint/no-explicit-any": "off",
    // "@typescript-eslint/no-inferrable-types": "off",
    // "@typescript-eslint/no-non-null-assertion": "off",
    // "vue/script-setup-uses-vars": "error",
    // "vue/v-on-event-hyphenation": "off",
    "vue/multi-word-component-names": "off",
    // "vue/no-reserved-component-names": "off",
  },
};
