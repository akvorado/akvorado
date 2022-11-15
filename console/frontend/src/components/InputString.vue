<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <InputBase v-slot="{ id, childClass }">
    <input
      :id="id"
      :class="childClass"
      type="text"
      placeholder=" "
      :value="modelValue"
      @input="
        $emit('update:modelValue', ($event.target as HTMLInputElement).value)
      "
    />
  </InputBase>
</template>

<script lang="ts" setup>
import InputBase from "@/components/InputBase.vue";

defineProps<{
  modelValue: string;
}>();
defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();
</script>
