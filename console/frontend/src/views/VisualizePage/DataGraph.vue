<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <component
    :is="component"
    :theme="isDark ? 'dark' : null"
    :data="data"
    autoresize
  />
</template>

<script setup>
const props = defineProps({
  data: {
    type: Object,
    default: null,
  },
});

import { computed, inject } from "vue";
import { graphTypes } from "./constants";
import DataGraphTimeSeries from "./DataGraphTimeSeries.vue";
import DataGraphSankey from "./DataGraphSankey.vue";
const { isDark } = inject("theme");

const component = computed(() => {
  const { stacked, lines, grid, sankey } = graphTypes;
  if ([stacked, lines, grid].includes(props.data.graphType)) {
    return DataGraphTimeSeries;
  }
  if ([sankey].includes(props.data.graphType)) {
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
