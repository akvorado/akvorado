<template>
  <div class="relative">
    <div
      v-if="loading"
      class="absolute inset-0 z-40 flex h-full w-full items-start justify-center bg-white/80 pt-10 dark:bg-gray-800/80"
    >
      <div
        class="fixed flex flex-col items-center rounded-lg border border-gray-300 bg-gray-200/80 p-5 shadow-lg dark:border-gray-700 dark:bg-gray-600/80 dark:shadow-white/10"
      >
        <LoadingSpinner class="block w-10" />
        <div class="mt-3">Loading...</div>
      </div>
    </div>
    <slot></slot>
  </div>
</template>

<script setup>
defineProps({
  loading: {
    type: Boolean,
    required: true,
  },
});

import LoadingSpinner from "@/components/LoadingSpinner.vue";
</script>
