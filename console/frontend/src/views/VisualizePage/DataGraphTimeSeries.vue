<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

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
const emit = defineEmits(["update:timeRange"]);

import { ref, watch, inject, computed, onMounted, nextTick } from "vue";
import { useMediaQuery } from "@vueuse/core";
import { formatXps, dataColor, dataColorGrey } from "@/utils";
import { graphTypes } from "./constants";
const { isDark } = inject("theme");

import { uniqWith, isEqual, findIndex } from "lodash-es";
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
};
const graph = computed(() => {
  const theme = isDark.value ? "dark" : "light";
  const data = props.data || {};
  if (!data.t) return {};
  const rowName = (row) => row.join(" — ") || "Total",
    dataset = {
      sourceHeader: false,
      dimensions: ["time", ...data.rows.map(rowName)],
      source: [
        ...data.t
          .map((t, timeIdx) => [
            t,
            ...data.points.map(
              // Unfortunately, eCharts does not seem to make it easy
              // to inverse an axis and put the result below. Therefore,
              // we use negative values for the second axis.
              (row, rowIdx) => row[timeIdx] * (data.axis[rowIdx] % 2 ? 1 : -1)
            ),
          ])
          .slice(0, -1),
      ],
    },
    xAxis = {
      type: "time",
      min: data.start,
      max: data.end,
    },
    yAxis = {
      type: "value",
      min: data.bidirectional ? undefined : 0,
      axisLabel: { formatter: formatXps },
      axisPointer: {
        label: { formatter: ({ value }) => formatXps(value) },
      },
    },
    tooltip = {
      confine: true,
      trigger: "axis",
      axisPointer: {
        type: "cross",
        label: { backgroundColor: "#6a7985" },
      },
      backgroundColor: isDark.value ? "#222e" : "#eeee",
      textStyle: isDark.value ? { color: "#ddd" } : { color: "#222" },
      formatter: (params) => {
        // We will use a custom formatter, notably to handle bidirectional tooltips.
        if (params.length === 0) return;

        let table = [];
        params.forEach((param) => {
          const axis = data.axis[param.seriesIndex];
          const seriesName = [1, 2].includes(axis)
            ? param.seriesName
            : data["axis-names"][axis];
          const key = `${Math.floor((axis - 1) / 2)}-${seriesName}`;
          let idx = findIndex(table, (r) => r.key === key);
          if (idx === -1) {
            table.push({
              key,
              seriesName,
              marker: param.marker,
              up: 0,
              down: 0,
            });
            idx = table.length - 1;
          }
          const val = param.value[param.seriesIndex + 1];
          if (axis % 2 == 1) table[idx].up = val;
          else table[idx].down = val;
        });
        const rows = table
          .map((row) =>
            [
              `<tr>`,
              `<td>${row.marker} ${row.seriesName}</td>`,
              `<td class="pl-2">${data.bidirectional ? "↑" : ""}<b>${formatXps(
                row.up || 0
              )}</b></td>`,
              data.bidirectional
                ? `<td class="pl-2">↓<b>${formatXps(row.down || 0)}</b></td>`
                : "",
              `</tr>`,
            ].join("")
          )
          .join("");
        return `${params[0].axisValueLabel}<table>${rows}</table>`;
      },
    };

  // Lines and stacked areas
  if ([graphTypes.stacked, graphTypes.lines].includes(data.graphType)) {
    const uniqRows = uniqWith(data.rows, isEqual),
      uniqRowIndex = (row) => findIndex(uniqRows, (orow) => isEqual(row, orow));

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
      tooltip,
      series: data.rows
        .map((row, idx) => {
          const isOther = row.some((name) => name === "Other"),
            color = isOther ? dataColorGrey : dataColor;
          if (data.graphType === graphTypes.lines && isOther) {
            return undefined;
          }
          let serie = {
            type: "line",
            symbol: "none",
            itemStyle: {
              color: color(uniqRowIndex(row), false, theme),
            },
            lineStyle: {
              color: color(uniqRowIndex(row), false, theme),
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
          if ([3, 4].includes(data.axis[idx])) {
            serie = {
              ...serie,
              itemStyle: {
                color: dataColorGrey(1, false, theme),
              },
              lineStyle: {
                color: dataColorGrey(1, false, theme),
                shadowColor: "#000",
                shadowOffsetX: 1,
                shadowOffsetY: 1,
                shadowBlur: 2,
                width: 2,
                type: "dashed",
              },
            };
          }
          if (
            data.graphType === graphTypes.stacked &&
            [1, 2].includes(data.axis[idx])
          ) {
            serie = {
              ...serie,
              stack: data.axis[idx],
              lineStyle:
                idx == data.rows.length - 1 ||
                data.axis[idx] != data.axis[idx + 1]
                  ? {
                      color: isDark.value ? "#ddd" : "#111",
                      width: 1.5,
                    }
                  : {
                      color: color(uniqRowIndex(row), false, theme),
                      width: 1,
                    },
              areaStyle: {
                opacity: 0.95,
                color: new graphic.LinearGradient(0, 0, 0, 1, [
                  { offset: 0, color: color(uniqRowIndex(row), false, theme) },
                  { offset: 1, color: color(uniqRowIndex(row), true, theme) },
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
    const uniqRows = uniqWith(data.rows, isEqual).filter((row) =>
        row.some((name) => name !== "Other")
      ),
      uniqRowIndex = (row) => findIndex(uniqRows, (orow) => isEqual(row, orow)),
      otherIndexes = data.rows
        .map((row, idx) => (row.some((name) => name === "Other") ? idx : -1))
        .filter((idx) => idx >= 0),
      somethingY = (fn) =>
        fn(
          ...dataset.source.map((row) =>
            fn(
              ...row
                .slice(1)
                .filter((_, idx) => !otherIndexes.includes(idx + 1))
            )
          )
        ),
      maxY = somethingY(Math.max),
      minY = somethingY(Math.min);
    let rowNumber = Math.ceil(Math.sqrt(uniqRows.length)),
      colNumber = rowNumber;
    if ((rowNumber - 1) * colNumber >= uniqRows.length) {
      rowNumber--;
    }
    const positions = uniqRows.map((_, idx) => ({
      left: ((idx % colNumber) / colNumber) * 100,
      top: (Math.floor(idx / colNumber) / rowNumber) * 100,
      width: (1 / colNumber) * 100,
      height: (1 / rowNumber) * 100,
    }));
    return {
      title: uniqRows.map((_, idx) => ({
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
      grid: uniqRows.map((_, idx) => ({
        show: true,
        borderWidth: 0,
        left: positions[idx].left + 0.25 + "%",
        top: positions[idx].top + 0.25 + "%",
        width: positions[idx].width - 0.5 + "%",
        height: positions[idx].height - 0.5 + "%",
      })),
      xAxis: uniqRows.map((_, idx) => ({
        ...xAxis,
        gridIndex: idx,
        show: false,
      })),
      yAxis: uniqRows.map((_, idx) => ({
        ...yAxis,
        max: maxY,
        min: data.bidirectional ? minY : 0,
        gridIndex: idx,
        show: false,
      })),
      dataset,
      series: data.rows
        .map((row, idx) => {
          let serie = {
            type: "line",
            symbol: "none",
            xAxisIndex: uniqRowIndex(row),
            yAxisIndex: uniqRowIndex(row),
            itemStyle: {
              color: dataColor(uniqRowIndex(row), false, theme),
            },
            areaStyle: {
              opacity: 0.95,
              color: new graphic.LinearGradient(0, 0, 0, 1, [
                {
                  offset: 0,
                  color: dataColor(uniqRowIndex(row), false, theme),
                },
                {
                  offset: 1,
                  color: dataColor(uniqRowIndex(row), true, theme),
                },
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
        })
        .filter((s) => s.xAxisIndex >= 0),
    };
  }
  return {};
});
const echartsOptions = computed(() => ({ ...commonGraph, ...graph.value }));

// Enable and handle brush
const isTouchScreen = useMediaQuery("(pointer: coarse");
const enableBrush = () => {
  nextTick().then(() => {
    chartComponent.value?.dispatchAction({
      type: "takeGlobalCursor",
      key: "brush",
      brushOption: {
        brushType: isTouchScreen.value ? false : "lineX",
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
  emit("update:timeRange", [start, end]);
};
watch([graph, isTouchScreen], enableBrush);

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
