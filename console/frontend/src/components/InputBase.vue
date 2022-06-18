<template>
  <div>
    <div class="relative">
      <slot
        :id="id"
        :child-class="{
          'peer block w-full appearance-none rounded-t-lg border-0 border-b-2 bg-white px-2.5 py-1.5 text-sm text-gray-900 focus-withinoutline-none focus-withinring-0 dark:bg-gray-900 dark:text-white': true,
          'pt-4': label,
          'dark:focus-withinborder-red-500 dark:focus-within:border-red-500 border-red-600 focus-withinborder-red-600 focus-withing:border-red-600':
            error,
          'border-gray-300 focus-withinborder-blue-600 focus-within:border-blue-600 dark:border-gray-600 dark:focus-withinborder-blue-500 dark:focus-within:border-blue-500':
            !error,
        }"
      />
      <label
        v-if="label"
        :for="id"
        :class="{
          'text-red-600 dark:text-red-500': error,
          'peer-focus-withintext-blue-600 dark:peer-focus-withintext-blue-500 text-gray-500 dark:text-gray-400':
            !error,
        }"
        class="z-5 peer-focus-within-translate-y-3 peer-focus-withinscale-75 absolute top-3 left-2.5 origin-[0] -translate-y-3 scale-75 transform font-sans text-sm duration-300 peer-placeholder-shown:translate-y-0 peer-placeholder-shown:scale-100"
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
