import pluginJs from "@eslint/js";
import pluginVue from "eslint-plugin-vue";
import vueTypescriptEslintConfig from "@vue/eslint-config-typescript";
import vuePrettierEslintConfig from "@vue/eslint-config-prettier";

export default [
  {
    languageOptions: {
      ecmaVersion: 2021,
    },
  },
  pluginJs.configs.recommended,
  ...pluginVue.configs["flat/recommended"],
  ...vueTypescriptEslintConfig(),
  vuePrettierEslintConfig,
];
