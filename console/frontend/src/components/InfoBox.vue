<template>
  <div class="my-4 flex rounded-lg p-4 text-sm" :class="classes" role="alert">
    <InformationCircleIcon class="mr-2 h-5 w-5 shrink-0" />
    <span><slot></slot></span>
  </div>
</template>

<script setup>
const props = defineProps({
  kind: {
    type: String,
    default: "error",
    validator(value) {
      return ["info", "danger", "success", "warning"].includes(value);
    },
  },
});

import { computed } from "vue";
import { InformationCircleIcon } from "@heroicons/vue/solid";

const classes = computed(() => {
  switch (props.kind) {
    case "info":
      return "text-blue-700 bg-blue-100 dark:bg-blue-200 dark:text-blue-800";
    case "danger":
      return "text-red-700 bg-red-100 dark:bg-red-200 dark:text-red-800";
    case "success":
      return "text-green-700 bg-green-100 dark:bg-green-200 dark:text-green-800";
    case "warning":
      return "text-yellow-700 bg-yellow-100 dark:bg-yellow-200 dark:text-yellow-800";
  }
  return "";
});
</script>
