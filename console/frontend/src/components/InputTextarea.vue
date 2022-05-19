<template>
  <InputComponent v-slot="{ id, childClass }">
    <textarea
      :id="id"
      :class="childClass"
      class="resize-none appearance-none"
      :rows="rows"
      placeholder=" "
      :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)"
    />
  </InputComponent>
</template>

<script setup>
defineProps({
  rows: {
    type: Number,
    default: 4,
  },
  modelValue: {
    type: String,
    required: true,
  },
});
defineEmits(["update:modelValue"]);

import InputComponent from "@/components/InputComponent.vue";
</script>
