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
      @input="$emit('update:modelValue', $event.target.value)"
    />
  </InputBase>
</template>

<script setup>
defineProps({
  modelValue: {
    type: String,
    required: true,
  },
});
defineEmits(["update:modelValue"]);

import InputBase from "@/components/InputBase.vue";
</script>
