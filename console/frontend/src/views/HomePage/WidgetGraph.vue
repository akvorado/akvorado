<template>
  <div>
    <div class="h-[300px]">
      <v-chart :option="option" :theme="isDark() ? 'dark' : null" autoresize />
    </div>
  </div>
</template>

<script setup>
import { ref, watch, inject } from "vue";
import { use, graphic } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { LineChart } from "echarts/charts";
import { TooltipComponent, GridComponent } from "echarts/components";
import VChart from "vue-echarts";
import { dataColor, formatBps } from "../../utils";
const { isDark } = inject("darkMode");

use([CanvasRenderer, LineChart, TooltipComponent, GridComponent]);

const formatGbps = (value) => formatBps(value * 1_000_000_000);

const props = defineProps({
  refresh: {
    type: Number,
    required: true,
  },
});
const option = ref({
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
        width: 1,
      },
      data: [],
    },
  ],
});

watch(
  isDark,
  (isDark) => {
    const theme = isDark ? "dark" : "light";
    option.value.darkMode = isDark;
    option.value.series[0].areaStyle = {
      opacity: 0.9,
      color: new graphic.LinearGradient(0, 0, 0, 1, [
        { offset: 0, color: dataColor(0, false, theme) },
        { offset: 1, color: dataColor(0, true, theme) },
      ]),
    };
    option.value.series[0].lineStyle.color = dataColor(0, false, theme);
  },
  { immediate: true }
);

watch(
  () => props.refresh,
  async () => {
    const response = await fetch("/api/v0/console/widget/graph");
    if (!response.ok) {
      // Keep current data
      return;
    }
    const data = await response.json();
    option.value.series[0].data = data.data
      .map(({ t, gbps }) => [t, gbps])
      .slice(1, -1);
  },
  { immediate: true }
);
</script>
