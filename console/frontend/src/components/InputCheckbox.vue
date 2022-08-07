<template>
  <div>
    <label :for="id" class="flex items-center">
      <input
        :id="id"
        type="checkbox"
        :checked="modelValue"
        class="h-4 w-4 rounded border-gray-300 bg-gray-100 text-blue-600 focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:ring-offset-gray-800 dark:focus:ring-blue-600"
        @change="$emit('update:modelValue', $event.target.checked)"
      />
      <span class="ml-1 text-sm font-medium text-gray-900 dark:text-gray-300">
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
