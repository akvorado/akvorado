module.exports = {
  env: {
    node: true,
    "vue/setup-compiler-macros": true,
  },
  parserOptions: {
    ecmaVersion: 2021,
  },
  extends: ["plugin:vue/vue3-recommended", "eslint:recommended", "prettier"],
  rules: {
    "vue/no-unused-vars": "error",
    "vue/no-v-html": "off",
  },
};
