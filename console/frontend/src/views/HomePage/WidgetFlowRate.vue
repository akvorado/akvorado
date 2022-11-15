<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="flex flex-col items-center justify-center">
    <h2
      class="title-font text-3xl font-medium text-gray-900 dark:text-gray-200"
    >
      {{ rate }}
    </h2>
    <p class="leading-relaxed">Flows/s</p>
  </div>
</template>

<script lang="ts" setup>
import { computed } from "vue";
import { useFetch } from "@vueuse/core";

const props = withDefaults(
  defineProps<{
    refresh?: number;
  }>(),
  {
    refresh: 0,
  }
);

const url = computed(() => `/api/v0/console/widget/flow-rate?${props.refresh}`);
const { data } = useFetch(url, { refetch: true }).get().json<{
  rate: number;
  period: string;
}>();
const rate = computed(() => {
  if (!data.value?.rate) {
    return "???";
  }
  if (data.value?.rate > 1_500_000) {
    return (data.value.rate / 1_000_000).toFixed(1) + "M";
  }
  if (data.value?.rate > 1_500) {
    return (data.value.rate / 1_000).toFixed(1) + "K";
  }
  return data.value.rate.toFixed(0);
});
</script>
