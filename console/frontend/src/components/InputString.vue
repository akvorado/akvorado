<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <InputBase v-slot="{ id, childClass }">
    <input
      :id="id"
      :class="[childClass, { 'pr-8': $slots.button }]"
      type="text"
      placeholder=" "
      :value="modelValue"
      @input="
        $emit('update:modelValue', ($event.target as HTMLInputElement).value)
      "
    />
    <span
      v-if="$slots.button"
      class="absolute inset-y-0 right-0 flex items-center pr-2"
    >
      <slot name="button"></slot>
    </span>
  </InputBase>
</template>

<script lang="ts" setup>
import InputBase from "@/components/InputBase.vue";

defineProps<{
  modelValue: string;
}>();
defineEmits<{
  "update:modelValue": [value: string];
}>();
</script>
