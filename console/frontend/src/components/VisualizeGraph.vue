<template>
  <v-chart :option="graph" autoresize />
</template>

<script setup>
import { ref, watch, inject } from "vue";
import { formatBps, dataColor, dataColorGrey } from "../utils";
const { isDark } = inject("darkMode");

import { use, graphic } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { LineChart } from "echarts/charts";
import { TooltipComponent, GridComponent } from "echarts/components";
import VChart from "vue-echarts";
use([CanvasRenderer, LineChart, TooltipComponent, GridComponent]);

const props = defineProps({
  data: {
    type: Object,
    default: () => {},
  },
});

const graph = ref({
  grid: {
    left: 60,
    top: 20,
    right: "1%",
    bottom: 20,
  },
  xAxis: {
    type: "time",
  },
  yAxis: {
    type: "value",
    min: 0,
    axisLabel: { formatter: formatBps },
    axisPointer: { label: { formatter: ({ value }) => formatBps(value) } },
  },
  tooltip: {
    confine: true,
    trigger: "axis",
    axisPointer: {
      type: "cross",
      label: { backgroundColor: "#6a7985" },
    },
    valueFormatter: formatBps,
  },
  series: [],
});

watch(
  () => [props.data, isDark()],
  ([data, isDark]) => {
    if (data.t === undefined) {
      return;
    }
    const theme = isDark ? "dark" : "light";

    graph.value.darkMode = isDark;
    graph.value.xAxis.data = data.t.slice(1, -1);
    graph.value.series = data.rows.map((rows, idx) => {
      const color = rows.some((name) => name === "Other")
        ? dataColorGrey
        : dataColor;
      return {
        type: "line",
        name: rows.join(" — "),
        symbol: "none",
        itemStyle: {
          color: color(idx, false, theme),
        },
        lineStyle:
          idx == data.rows.length - 1
            ? {
                color: isDark ? "#ddd" : "#111",
                width: 2,
              }
            : {
                color: color(idx, false, theme),
                width: 1,
              },
        areaStyle: {
          opacity: 0.95,
          color: new graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: color(idx, false, theme) },
            { offset: 1, color: color(idx, true, theme) },
          ]),
        },
        emphasis: {
          focus: "series",
        },
        stack: "all",
        data: data.t.map((t, idx2) => [t, data.points[idx][idx2]]).slice(1, -1),
      };
    });
  },
  { immediate: true }
);
</script>
