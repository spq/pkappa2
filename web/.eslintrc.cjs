module.exports = {
  root: true,
  env: {
    es2021: true,
  },
  parserOptions: {
    ecmaVersion: 2021,
  },
  extends: [
    // add more generic rulesets here, such as:
    "eslint:recommended",
    "plugin:vue/recommended",
    "prettier",
  ],
  rules: {
    "no-console": "off",
    "no-debugger": process.env.NODE_ENV === "production" ? "warn" : "off",
    "vue/multi-word-component-names": "off",
    "vue/no-reserved-component-names": "off",
  },
  overrides: [
    {
      files: ["**/*.ts", "**/*.vue"],
      parser: "vue-eslint-parser",
      env: {
        es2021: true,
      },
      parserOptions: {
        project: "./tsconfig.json",
        parser: "@typescript-eslint/parser",
      },
      plugins: ["@typescript-eslint", "@typescript-eslint/eslint-plugin"],
      extends: [
        "eslint:recommended",
        "plugin:vue/recommended", // Use this if you are using Vue.js 2.x.
        "@vue/typescript",
        "plugin:@typescript-eslint/recommended-requiring-type-checking",
        "plugin:@typescript-eslint/eslint-recommended",
        "plugin:@typescript-eslint/recommended",
        "prettier",
      ],
      rules: {
        "no-console": "off",
        "no-debugger": process.env.NODE_ENV === "production" ? "warn" : "off",
        "vue/multi-word-component-names": "off",
        "vue/no-reserved-component-names": "off",
        "@typescript-eslint/no-non-null-assertion": "off",
      },
    },
  ],
};
