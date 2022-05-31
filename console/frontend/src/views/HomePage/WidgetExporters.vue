<template>
  <div>
    <h2
      class="title-font text-3xl font-medium text-gray-900 dark:text-gray-200"
    >
      {{ exporters }}
    </h2>
    <p class="leading-relaxed">Exporters</p>
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

const url = computed(() => "/api/v0/console/widget/exporters?" + props.refresh);
const { data } = useFetch(url, { refetch: true }).get().json();
const exporters = computed(() => data?.value?.exporters?.length || "???");
</script>
