<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div>
    <div class="h-[300px]">
      <v-chart :option="options" :theme="isDark ? 'dark' : null" autoresize />
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  refresh: {
    type: Number,
    required: true,
  },
});

import { computed, inject } from "vue";
import { useFetch } from "@vueuse/core";
import { use, graphic } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { LineChart } from "echarts/charts";
import { TooltipComponent, GridComponent } from "echarts/components";
import VChart from "vue-echarts";
import { dataColor, formatXps } from "../../utils";
const { isDark } = inject("theme");

use([CanvasRenderer, LineChart, TooltipComponent, GridComponent]);

const formatGbps = (value) => formatXps(value * 1_000_000_000);

const url = computed(() => `/api/v0/console/widget/graph?${props.refresh}`);
const { data } = useFetch(url, { refetch: true }).get().json();
const options = computed(() => ({
  darkMode: isDark.value,
  backgroundColor: "transparent",
  xAxis: { type: "time" },
  yAxis: {
    type: "value",
    min: 0,
    axisLabel: { formatter: formatGbps },
  },
  tooltip: {
    confine: true,
    trigger: "axis",
    axisPointer: {
      type: "cross",
      label: { backgroundColor: "#6a7985" },
    },
    valueFormatter: formatGbps,
  },
  series: [
    {
      type: "line",
      symbol: "none",
      lineStyle: {
        width: 0,
      },
      areaStyle: {
        opacity: 0.9,
        color: new graphic.LinearGradient(0, 0, 0, 1, [
          {
            offset: 0,
            color: dataColor(0, false, isDark.value ? "dark" : "light"),
          },
          {
            offset: 1,
            color: dataColor(0, true, isDark.value ? "dark" : "light"),
          },
        ]),
      },
      data: (data.value?.data || [])
        .map(({ t, gbps }) => [t, gbps])
        .slice(0, -1),
    },
  ],
}));
</script>
