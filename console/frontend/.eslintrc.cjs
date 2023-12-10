// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

module.exports = {
  env: {
    node: true,
    "vue/setup-compiler-macros": true,
  },
  parserOptions: {
    ecmaVersion: 2021,
  },
  extends: [
    "plugin:vue/vue3-recommended",
    "eslint:recommended",
    "@vue/eslint-config-typescript",
    "@vue/eslint-config-prettier",
  ],
  rules: {
    "vue/no-unused-vars": "error",
    "vue/no-v-html": "off",
  },
};
