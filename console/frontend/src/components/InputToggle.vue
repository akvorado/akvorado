<template>
  <div>
    <label :for="id" class="relative inline-flex cursor-pointer items-center">
      <input
        :id="id"
        type="checkbox"
        :checked="modelValue"
        class="peer sr-only"
        @change="$emit('update:modelValue', $event.target.checked)"
      />
      <div
        class="peer h-5 w-9 rounded-full bg-gray-200 after:absolute after:top-[2px] after:left-[2px] after:h-4 after:w-4 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-blue-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:border-gray-600 dark:bg-gray-700 dark:peer-focus:ring-blue-800"
      ></div>
      <span class="ml-3 text-sm font-medium text-gray-900 dark:text-gray-300">
        {{ label }}
      </span>
    </label>
  </div>
</template>

<script setup>
defineProps({
  label: {
    type: String,
    required: true,
  },
  modelValue: {
    type: Boolean,
    required: true,
  },
});
defineEmits(["update:modelValue"]);

import { v4 as uuidv4 } from "uuid";
const id = uuidv4();
</script>
