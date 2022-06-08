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
