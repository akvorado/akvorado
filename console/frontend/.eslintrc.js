module.exports = {
  env: {
    node: true,
  },
  extends: ["plugin:vue/vue3-recommended", "eslint:recommended", "prettier"],
  rules: {
    "vue/no-unused-vars": "error",
    "vue/no-v-html": "off",
  },
};
