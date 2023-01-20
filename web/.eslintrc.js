module.exports = {
    root: true,
    env: {
        node: true,
        'vue/setup-compiler-macros': true
    },
    'extends': [
        'plugin:vue/vue3-recommended',
        'eslint:recommended',
        '@vue/typescript/recommended',
        'plugin:@typescript-eslint/recommended-requiring-type-checking',
        "plugin:@typescript-eslint/eslint-recommended",
        "plugin:@typescript-eslint/recommended",
        'plugin:vue/base',
    ],
    ignorePatterns: [
        '**/*.guard.ts',
        '**/*.js',
        'capacitor.config.ts',
    ],
    plugins: ['@typescript-eslint', "@typescript-eslint/eslint-plugin"],
    parser: 'vue-eslint-parser',
    parserOptions: {
        ecmaVersion: 2020,
        project: './tsconfig.json',
        parser: "@typescript-eslint/parser",
    },
    rules: {
        'no-console': 'off',
        'no-debugger': process.env.NODE_ENV === 'production' ? 'warn' : 'off',
        'vue/no-deprecated-slot-attribute': 'off',
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/no-inferrable-types': 'off',
        '@typescript-eslint/no-non-null-assertion': 'off',
        'vue/script-setup-uses-vars': 'error',
        'vue/v-on-event-hyphenation': 'off',
        'vue/multi-word-component-names': 'off',
        'vue/no-reserved-component-names': 'off',
    },
    overrides: [
        {
            files: [
                '**/__tests__/*.{j,t}s?(x)',
                '**/tests/unit/**/*.spec.{j,t}s?(x)'
            ],
            env: {
                jest: true
            }
        },
    ]
}
