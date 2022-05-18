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
import { ref, watch } from "vue";

const props = defineProps({
  refresh: {
    type: Number,
    default: 0,
  },
});
const exporters = ref("???");

watch(
  () => props.refresh,
  async () => {
    const response = await fetch("/api/v0/console/widget/exporters");
    if (!response.ok) {
      // Keep current data
      return;
    }
    const data = await response.json();
    exporters.value = data.exporters.length;
  },
  { immediate: true }
);
</script>
