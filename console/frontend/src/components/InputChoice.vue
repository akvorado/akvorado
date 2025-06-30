<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="inline-flex items-center gap-1">
    <span class="text-sm lg:hidden">{{ label }}</span>
    <div
      class="inline-flex rounded-md shadow-xs dark:shadow-white/10"
      role="group"
    >
      <label
        v-for="(choice, idx) in choices"
        :key="choice.name"
        :for="id(choice.name)"
        class="cursor-pointer first:rounded-l-md last:rounded-r-md focus-within:z-10 focus-within:ring-2 focus-within:ring-blue-300 dark:focus-within:ring-blue-800"
      >
        <input
          :id="id(choice.name)"
          type="radio"
          :checked="modelValue === choice.name"
          class="peer sr-only"
          @change="
            ($event.target as HTMLInputElement).checked &&
            $emit('update:modelValue', choice.name)
          "
        />
        <div
          :class="{
            'rounded-l-md border-l': idx === 0,
            'rounded-r-md border-r': idx === choices.length - 1,
          }"
          class="border-b border-t border-gray-200 bg-white px-1 py-0.5 text-sm font-medium text-gray-900 hover:bg-gray-100 hover:text-blue-700 peer-checked:bg-blue-700 peer-checked:bg-blue-700 peer-checked:text-white peer-checked:hover:bg-blue-800 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:hover:bg-gray-600 dark:hover:text-white peer-checked:dark:bg-blue-600 peer-checked:dark:hover:bg-blue-700 lg:px-0.5 lg:tracking-tighter"
        >
          {{ choice.label }}
        </div>
      </label>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { v4 as uuidv4 } from "uuid";

defineProps<{
  label: string;
  choices: Array<{ name: string; label: string }>;
  modelValue: string;
}>();
defineEmits<{
  "update:modelValue": [value: string];
}>();

const baseID = uuidv4();
const id = (name: string) => `${baseID}-${name}`;
</script>
