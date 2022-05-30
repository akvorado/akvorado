<template>
  <div class="inline-flex items-center gap-2">
    <span class="text-sm">{{ label }}</span>
    <div
      class="inline-flex rounded-md shadow-sm dark:shadow-white/10"
      role="group"
    >
      <label
        v-for="({ name, label: blabel }, idx) in choices"
        :key="name"
        :for="id(name)"
        class="first:rounded-l-lg last:rounded-r-lg focus-within:z-10 focus-within:ring-2 focus-within:ring-blue-300 dark:focus-within:ring-blue-800"
      >
        <input
          :id="id(name)"
          type="radio"
          :checked="modelValue === name"
          class="peer sr-only"
          @change="$event.target.checked && $emit('update:modelValue', name)"
        />
        <div
          :class="{
            'rounded-l-md border-l': idx === 0,
            'rounded-r-md border-r': idx === choices.length - 1,
          }"
          class="border-t border-b border-gray-200 bg-white py-0.5 px-2 text-sm font-medium text-gray-900 hover:bg-gray-100 hover:text-blue-700 peer-checked:bg-blue-700 peer-checked:bg-blue-700 peer-checked:font-bold peer-checked:text-white peer-checked:hover:bg-blue-800 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:hover:bg-gray-600 dark:hover:text-white peer-checked:dark:bg-blue-600 peer-checked:dark:hover:bg-blue-700"
        >
          {{ blabel }}
        </div>
      </label>
    </div>
  </div>
</template>

<script setup>
defineProps({
  label: {
    type: String,
    required: true,
  },
  choices: {
    type: Array,
    required: true,
  },
  modelValue: {
    type: String,
    required: true,
  },
});
defineEmits(["update:modelValue"]);

import { v4 as uuidv4 } from "uuid";
const baseID = uuidv4();
const id = (name) => `${baseID}-${name}`;
</script>
