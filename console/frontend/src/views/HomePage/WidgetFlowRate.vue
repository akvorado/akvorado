<template>
  <div>
    <h2
      class="title-font text-3xl font-medium text-gray-900 dark:text-gray-200"
    >
      {{ rate }}
    </h2>
    <p class="leading-relaxed">Flows/s</p>
  </div>
</template>

<script setup>
const props = defineProps({
  refresh: {
    type: Number,
    default: 0,
  },
});

import { computed } from "vue";
import { useFetch } from "@vueuse/core";

const url = computed(() => "/api/v0/console/widget/flow-rate?" + props.refresh);
const { data } = useFetch(url, { refetch: true }).get().json();
const rate = computed(() => {
  if (data.value?.rate > 1_500_000) {
    return (data.value.rate / 1_000_000).toFixed(1) + "M";
  }
  if (data.value?.rate > 1_500) {
    return (data.value.rate / 1_000).toFixed(1) + "K";
  }
  if (data.value?.rate >= 0) {
    return data.value.rate.toFixed(0);
  }
  return "???";
});
</script>
