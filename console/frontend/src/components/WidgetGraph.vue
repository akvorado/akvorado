<template>
  <div>
    <div class="h-[300px]">
      <v-chart :option="option" autoresize />
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from "vue";
import { use, graphic } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { LineChart } from "echarts/charts";
import { TooltipComponent, GridComponent } from "echarts/components";
import VChart from "vue-echarts";

use([CanvasRenderer, LineChart, TooltipComponent, GridComponent]);

const formatGbps = (value) => {
  const suffixes = ["", "K", "M", "G", "T"];
  let idx = 0;
  value *= 1000 * 1000 * 1000;
  while (value >= 1000 && idx < suffixes.length) {
    value /= 1000;
    idx++;
  }
  value = value.toFixed(2);
  return `${value}${suffixes[idx]}`;
};

const props = defineProps({
  refresh: {
    type: Number,
    required: true,
  },
});
const option = ref({
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
        color: "#5470c6",
        width: 1,
      },
      areaStyle: {
        opacity: 1,
        color: new graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: "#5470c6" },
          { offset: 1, color: "#5572c8" },
        ]),
      },
      data: [],
    },
  ],
});

watch(
  () => props.refresh,
  async () => {
    const response = await fetch("/api/v0/console/widget/graph?width=200");
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
