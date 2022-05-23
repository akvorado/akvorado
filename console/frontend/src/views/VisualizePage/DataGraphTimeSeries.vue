<template>
  <v-chart
    ref="chartComponent"
    :option="echartsOptions"
    :update-options="{ notMerge: true }"
    @brush-end="updateTimeRange"
  />
</template>

<script setup>
const props = defineProps({
  data: {
    type: Object,
    default: () => {},
  },
  highlight: {
    type: Number,
    default: null,
  },
});
const emit = defineEmits(["updateTimeRange"]);

import { ref, watch, inject, computed, onMounted, nextTick } from "vue";
import { formatBps, dataColor, dataColorGrey } from "@/utils";
import { graphTypes } from "./constants";
const { isDark } = inject("theme");

import { use, graphic } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { LineChart } from "echarts/charts";
import {
  TooltipComponent,
  GridComponent,
  BrushComponent,
  ToolboxComponent,
  DatasetComponent,
  TitleComponent,
} from "echarts/components";
import VChart from "vue-echarts";
use([
  CanvasRenderer,
  LineChart,
  TooltipComponent,
  GridComponent,
  ToolboxComponent,
  BrushComponent,
  DatasetComponent,
  TitleComponent,
]);

// Graph component
const chartComponent = ref(null);
const commonGraph = {
  backgroundColor: "transparent",
  animationDuration: 500,
  toolbox: {
    show: false,
  },
  brush: {
    xAxisIndex: "all",
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
};
const graph = computed(() => {
  const theme = isDark.value ? "dark" : "light";
  const data = props.data || {};
  if (!data.t) return {};
  const dataset = {
      sourceHeader: false,
      dimensions: [
        "time",
        ...data.rows.map((rows) => rows.join(" — ") || "Total"),
      ],
      source: [
        ...data.t
          .map((t, timeIdx) => [t, ...data.points.map((rows) => rows[timeIdx])])
          .slice(1, -1),
      ],
    },
    xAxis = {
      type: "time",
      min: data.start,
      max: data.end,
    },
    yAxis = {
      type: "value",
      min: 0,
      axisLabel: { formatter: formatBps },
      axisPointer: {
        label: { formatter: ({ value }) => formatBps(value) },
      },
    };

  // Lines and stacked areas
  if ([graphTypes.stacked, graphTypes.lines].includes(data.graphType)) {
    return {
      grid: {
        left: 60,
        top: 20,
        right: "1%",
        bottom: 20,
      },
      xAxis,
      yAxis,
      dataset,
      series: data.rows
        .map((rows, idx) => {
          const isOther = rows.some((name) => name === "Other"),
            color = isOther ? dataColorGrey : dataColor;
          if (data.graphType === graphTypes.lines && isOther) {
            return undefined;
          }
          let serie = {
            type: "line",
            symbol: "none",
            itemStyle: {
              color: color(idx, false, theme),
            },
            lineStyle: {
              color: color(idx, false, theme),
              width: 2,
            },
            emphasis: {
              focus: "series",
            },
            encode: {
              x: 0,
              y: idx + 1,
              seriesName: idx + 1,
              seriesId: idx + 1,
            },
          };
          if (data.graphType === graphTypes.stacked) {
            serie = {
              ...serie,
              stack: "all",
              lineStyle:
                idx == data.rows.length - 1
                  ? {
                      color: isDark.value ? "#ddd" : "#111",
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
            };
          }
          return serie;
        })
        .filter((s) => s !== undefined),
    };
  }
  if (data.graphType === graphTypes.grid) {
    const dataRows = data.rows.filter((rows) =>
        rows.some((name) => name !== "Other")
      ),
      otherIndex = dataset.dimensions.indexOf("Other");
    const maxY = Math.max(
      ...dataset.source.map((rows) =>
        Math.max(...rows.slice(1).slice(0, otherIndex))
      )
    );
    let rowNumber = Math.ceil(Math.sqrt(dataRows.length)),
      colNumber = rowNumber;
    if ((rowNumber - 1) * colNumber >= dataRows.length) {
      rowNumber--;
    }
    const positions = dataRows.map((_, idx) => ({
      left: ((idx % colNumber) / colNumber) * 100,
      top: (Math.floor(idx / colNumber) / rowNumber) * 100,
      width: (1 / colNumber) * 100,
      height: (1 / rowNumber) * 100,
    }));
    return {
      title: dataRows.map((rows, idx) => ({
        textAlign: "left",
        textStyle: {
          fontSize: 12,
          fontWeight: "bold",
          textBorderWidth: 1,
          textBorderColor: isDark.value ? "#000a" : "#fffa",
        },
        text: dataset.dimensions[idx + 1],
        bottom: 100 - positions[idx].top - positions[idx].height - 0.5 + "%",
        left: positions[idx].left + 0.25 + "%",
      })),
      grid: dataRows.map((_, idx) => ({
        show: true,
        borderWidth: 0,
        left: positions[idx].left + 0.25 + "%",
        top: positions[idx].top + 0.25 + "%",
        width: positions[idx].width - 0.5 + "%",
        height: positions[idx].height - 0.5 + "%",
      })),
      xAxis: dataRows.map((_, idx) => ({
        ...xAxis,
        gridIndex: idx,
        show: false,
      })),
      yAxis: dataRows.map((_, idx) => ({
        ...yAxis,
        max: maxY,
        gridIndex: idx,
        show: false,
      })),
      dataset,
      series: dataRows.map((rows, idx) => {
        let serie = {
          type: "line",
          symbol: "none",
          xAxisIndex: idx,
          yAxisIndex: idx,
          itemStyle: {
            color: dataColor(idx, false, theme),
          },
          areaStyle: {
            opacity: 0.95,
            color: new graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: dataColor(idx, false, theme) },
              { offset: 1, color: dataColor(idx, true, theme) },
            ]),
          },
          emphasis: {
            focus: "series",
          },
          encode: {
            x: 0,
            y: idx + 1,
            seriesName: idx + 1,
            seriesId: idx + 1,
          },
        };
        return serie;
      }),
    };
  }
  return {};
});
const echartsOptions = computed(() => ({ ...commonGraph, ...graph.value }));

// Enable and handle brush
const enableBrush = () => {
  nextTick().then(() => {
    chartComponent.value?.dispatchAction({
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
  const [start, end] = evt.areas[0].coordRange.map((t) => new Date(t));
  chartComponent.value.dispatchAction({
    type: "brush",
    areas: [],
  });
  emit("updateTimeRange", [start, end]);
};
watch(graph, enableBrush);

// Highlight selected indexes
watch(
  () => [props.highlight, props.data],
  ([index]) => {
    chartComponent.value?.dispatchAction({
      type: "highlight",
      seriesIndex: index,
    });
  }
);
</script>
