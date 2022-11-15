<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <button
    :disabled="disabled"
    :type="attrType"
    :class="{
      'bg-blue-700 font-bold text-white hover:bg-blue-800 focus:ring-blue-300 dark:bg-blue-600 dark:hover:bg-blue-700 dark:focus:ring-blue-800':
        type === 'primary' && !disabled,
      'bg-blue-300 font-medium text-white dark:bg-blue-500':
        type === 'primary' && disabled,
      'bg-orange-500 font-bold text-white hover:bg-orange-600 focus:ring-orange-400 dark:focus:ring-orange-900':
        type === 'warning' && !disabled,
      'bg-orange-200 font-medium text-white': type === 'warning' && disabled,
      'bg-red-700 font-bold text-white hover:bg-red-800 focus:ring-red-300 dark:bg-red-600 dark:hover:bg-red-700 dark:focus:ring-red-900':
        type === 'danger' && !disabled,
      'bg-red-300 font-medium text-white dark:bg-red-600':
        type === 'danger' && disabled,
      'border border-gray-300 bg-white font-medium text-gray-900 hover:bg-gray-100 focus:ring-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:hover:border-gray-600 dark:hover:bg-gray-700 dark:focus:ring-gray-700':
        type === 'alternative' && !disabled,
      'border border-gray-300 bg-white font-medium text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white':
        type === 'alternative' && disabled,
      'cursor-not-allowed': disabled,
      'px-2 py-1 text-xs focus:ring-2': size === 'small',
      'px-5 py-2.5 text-sm focus:ring-4': size === 'normal',
    }"
    class="inline-flex cursor-pointer items-center rounded-lg text-center transition-colors duration-200 focus:outline-none"
  >
    <LoadingSpinner
      v-if="loading"
      :class="{
        'mr-2 h-4 w-4': size === 'normal',
        'mr-1 h-3 w-3': size === 'small',
      }"
    />
    <slot></slot>
  </button>
</template>

<script lang="ts" setup>
import LoadingSpinner from "@/components/LoadingSpinner.vue";

withDefaults(
  defineProps<{
    attrType?: "submit" | "button" | "reset";
    type?: "alternative" | "primary" | "warning" | "danger";
    size?: "normal" | "small";
    disabled?: boolean;
    loading?: boolean;
  }>(),
  {
    attrType: "button",
    type: "primary",
    size: "normal",
    disabled: false,
    floading: false,
  }
);
</script>
