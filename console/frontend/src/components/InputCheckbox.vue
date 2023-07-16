<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div>
    <label :for="id" class="flex items-center">
      <input
        :id="id"
        type="checkbox"
        :checked="modelValue"
        class="h-4 w-4 rounded border-gray-300 bg-gray-100 text-blue-600 focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:ring-offset-gray-800 dark:focus:ring-blue-600"
        @change="
          $emit(
            'update:modelValue',
            ($event.target as HTMLInputElement).checked,
          )
        "
      />
      <span class="ml-1 text-sm font-medium text-gray-900 dark:text-gray-300">
        {{ label }}
      </span>
    </label>
  </div>
</template>

<script lang="ts" setup>
import { v4 as uuidv4 } from "uuid";

defineProps<{
  label: string;
  modelValue: boolean;
}>();
defineEmits<{
  "update:modelValue": [value: boolean];
}>();

const id = uuidv4();
</script>
