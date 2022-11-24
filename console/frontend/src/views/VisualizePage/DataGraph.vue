<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <component
    :is="component"
    :theme="isDark ? 'dark' : undefined"
    :data="data"
    autoresize
  />
</template>

<script lang="ts" setup>
import { computed, inject } from "vue";
import DataGraphTimeSeries from "./DataGraphTimeSeries.vue";
import DataGraphSankey from "./DataGraphSankey.vue";
import type { GraphHandlerResult, SankeyHandlerResult } from ".";
import { ThemeKey } from "@/components/ThemeProvider.vue";
const { isDark } = inject(ThemeKey)!;

const props = defineProps<{
  data: GraphHandlerResult | SankeyHandlerResult | null;
}>();

const component = computed(() => {
  switch (props.data?.graphType) {
    case "stacked":
    case "stacked100":
    case "lines":
    case "grid":
      return DataGraphTimeSeries;
    case "sankey":
      return DataGraphSankey;
  }
  return "div";
});
</script>

<style scoped>
:deep(x-vue-echarts) > :deep(div:first-child) {
  width: auto !important;
}
</style>
