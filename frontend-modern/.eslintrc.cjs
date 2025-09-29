module.exports = {
  root: true,
  env: {
    browser: true,
    es2021: true,
    node: true,
  },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:solid/typescript',
    'prettier',
  ],
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
  },
  plugins: ['@typescript-eslint', 'solid'],
  ignorePatterns: ['dist', 'node_modules'],
  rules: {
    '@typescript-eslint/no-unused-vars': [
      'warn',
      {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
        caughtErrorsIgnorePattern: '^_',
      },
    ],
    '@typescript-eslint/no-unused-expressions': 'off',
    '@typescript-eslint/no-explicit-any': 'off',
    'no-case-declarations': 'off',
    'no-useless-escape': 'off',
    'prefer-const': 'off',
    'solid/reactivity': 'off',
    'solid/prefer-for': 'off',
    'solid/style-prop': 'off',
    'solid/components-return-once': 'off',
    'solid/self-closing-comp': 'off',
  },
};
