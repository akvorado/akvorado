<template>
  <div>
    <div class="relative">
      <slot
        :id="id"
        :child-class="{
          'peer block w-full appearance-none rounded-t-lg border-0 border-b-2 bg-gray-50 px-2.5 pb-1.5 pt-4 text-sm text-gray-900 focus:outline-none focus:ring-0 dark:bg-gray-700 dark:text-white': true,
          'dark:focus-border-red-500 border-red-600 focus:border-red-600':
            error,
          'border-gray-300 focus:border-blue-600 dark:border-gray-600 dark:focus:border-blue-500':
            !error,
        }"
      />
      <label
        :for="id"
        :class="{
          'text-red-600 dark:text-red-500': error,
          'text-gray-500 peer-focus:text-blue-600 dark:text-gray-400 dark:peer-focus:text-blue-500':
            !error,
        }"
        class="z-5 absolute top-3 left-2.5 origin-[0] -translate-y-3 scale-75 transform font-sans text-sm duration-300 peer-placeholder-shown:translate-y-0 peer-placeholder-shown:scale-100 peer-focus:-translate-y-3 peer-focus:scale-75"
        >{{ props.label }}</label
      >
    </div>
    <p class="text-xs text-red-600 dark:text-red-400">
      {{ props.error }}
    </p>
  </div>
</template>

<script setup>
const props = defineProps({
  label: {
    type: String,
    required: true,
  },
  error: {
    type: String,
    default: "",
  },
});

import { v4 as uuidv4 } from "uuid";

const id = uuidv4();
</script>
