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
import { ref, watch } from "vue";

const props = defineProps({
  refresh: {
    type: Number,
    default: 0,
  },
});
const rate = ref("???");

watch(
  () => props.refresh,
  async () => {
    const response = await fetch("/api/v0/console/widget/flow-rate");
    if (!response.ok) {
      rate.value = "???";
      return;
    }
    const data = await response.json();
    if (data.rate > 1_500_000) {
      rate.value = (data.rate / 1_000_000).toFixed(1) + "M";
    } else if (data.rate > 1_500) {
      rate.value = (data.rate / 1_000).toFixed(1) + "K";
    } else {
      rate.value = data.rate.toFixed(0);
    }
  },
  { immediate: true }
);
</script>
