<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div>
    <div class="relative">
      <slot
        :id="id"
        :child-class="{
          'peer block w-full appearance-none rounded-t-lg border-0 border-b-2 bg-white px-2.5 py-1.5 text-sm text-gray-900 focus-within:outline-none focus-within:ring-0 dark:bg-gray-900 dark:text-white': true,
          'pt-4': label,
          'dark:focus-within:border-red-500 border-red-600 focus-within:border-red-600':
            error,
          'border-gray-300 focus-within:border-blue-600 focus-within:border-blue-600 dark:border-gray-600 dark:focus-within:border-blue-500':
            !error,
        }"
      />
      <label
        v-if="label"
        :for="id"
        :class="{
          'text-red-600 dark:text-red-500': error,
          'text-gray-500 peer-focus-within:text-blue-600 dark:text-gray-400 dark:peer-focus-within:text-blue-500':
            !error,
        }"
        class="z-5 -peer-focus-within:translate-y-3 absolute top-3 left-2.5 origin-[0] -translate-y-3 scale-75 transform font-sans text-sm duration-300 peer-placeholder-shown:translate-y-0 peer-placeholder-shown:scale-100 peer-focus-within:scale-75"
        >{{ label }}</label
      >
    </div>
    <p class="text-xs text-red-600 dark:text-red-400">
      {{ error }}
    </p>
  </div>
</template>

<script setup>
defineProps({
  label: {
    type: String,
    default: null,
  },
  error: {
    type: String,
    default: "",
  },
});

import { v4 as uuidv4 } from "uuid";
const id = uuidv4();
</script>
