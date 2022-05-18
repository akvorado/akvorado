<template>
  <v-chart
    ref="chartComponent"
    :option="graph"
    :update-options="{ notMerge: true }"
    :loading="props.loading"
    :theme="isDark() ? 'dark' : null"
    autoresize
    @brush-end="updateTimeRange"
  />
</template>

<script setup>
import { ref, watch, inject, onMounted, nextTick } from "vue";
import { formatBps, dataColor, dataColorGrey } from "../utils";
const { isDark } = inject("darkMode");

import { use, graphic } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { LineChart } from "echarts/charts";
import {
  TooltipComponent,
  GridComponent,
  BrushComponent,
  ToolboxComponent,
} from "echarts/components";
import VChart from "vue-echarts";
use([
  CanvasRenderer,
  LineChart,
  TooltipComponent,
  GridComponent,
  ToolboxComponent,
  BrushComponent,
]);

const props = defineProps({
  data: {
    type: Object,
    default: () => {},
  },
  loading: {
    type: Boolean,
    default: false,
  },
  highlight: {
    type: Number,
    default: null,
  },
});
const emit = defineEmits(["updateTimeRange"]);

const chartComponent = ref(null);
const graph = ref({
  backgroundColor: "transparent",
  grid: {
    left: 60,
    top: 20,
    right: "1%",
    bottom: 20,
  },
  brush: {},
  toolbox: {
    show: false,
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
  animationDuration: 500,
  series: [],
});

const enableBrush = () => {
  nextTick().then(() => {
    if (!chartComponent.value) {
      return;
    }
    chartComponent.value.dispatchAction({
      type: "takeGlobalCursor",
      key: "brush",
      brushOption: {
        brushType: "lineX",
      },
    });
  });
};
onMounted(enableBrush);
const updateTimeRange = (evt) => {
  if (evt.areas.length === 0) {
    return;
  }
  const [start, end] = evt.areas[0].range.map(
    (px) => new Date(chartComponent.value.convertFromPixel("xAxis", px))
  );
  chartComponent.value.dispatchAction({
    type: "brush",
    areas: [],
  });
  emit("updateTimeRange", [start, end]);
};

watch(
  () => [props.data, isDark()],
  ([data, isDark]) => {
    if (data.t === undefined) {
      return;
    }
    const theme = isDark ? "dark" : "light";

    graph.value.xAxis.data = data.t.slice(1, -1);
    graph.value.xAxis.min = data.start;
    graph.value.xAxis.max = data.end;
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
    enableBrush();
  },
  { immediate: true }
);
watch(
  () => [props.highlight, props.data],
  ([index]) => {
    if (!chartComponent.value) {
      return;
    }
    chartComponent.value.dispatchAction({
      type: "highlight",
      seriesIndex: index,
    });
  }
);
</script>

<style scoped>
x-vue-echarts > :deep(div:first-child) {
  width: auto !important;
}
</style>
