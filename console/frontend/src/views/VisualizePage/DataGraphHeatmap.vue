<!-- SPDX-FileCopyrightText: 2026 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <v-chart
    ref="chartComponent"
    :option="option"
    :update-options="{ notMerge: true }"
    @brush-end="updateTimeRange"
  />
</template>

<script lang="ts" setup>
import { inject, computed, toRef } from "vue";
import { formatXps } from "@/utils";
import { ThemeKey } from "@/components/ThemeProvider.vue";
import type { GraphLineHandlerResult } from ".";
import VChart from "vue-echarts";
import {
  useTimeSeriesGraph,
  rowName,
  type ECOption,
} from "./useTimeSeriesGraph";

const PALETTE_MAGMA = ["#fcfdbf", "#fc8961", "#b73779", "#51127c", "#000004"];

const props = defineProps<{
  data: GraphLineHandlerResult;
  highlight: number | null;
}>();
const emit = defineEmits<{
  "update:timeRange": [range: [Date, Date]];
}>();

const { isDark } = inject(ThemeKey)!;

const graph = computed((): ECOption => {
  const data = props.data;
  if (!data) return {};

  const source: [string, number, number][] = data.points
    .map((row, rowIdx) => {
      const ret: [number, number[]] = [rowIdx, row];
      return ret;
    })
    .filter(([origRowIdx]) =>
      data.rows[origRowIdx].some((name) => name !== "Other"),
    )
    .toSorted(([origRowIdx1], [origRowIdx2]) => {
      return rowName(data.rows[origRowIdx1]).localeCompare(
        rowName(data.rows[origRowIdx2]),
        "en",
        { numeric: true },
      );
    })
    .flatMap(([origRowIdx, row], rowIdx) =>
      data.t.flatMap((t, timeIdx) => {
        const value = row[timeIdx] * (data.axis[origRowIdx] % 2 ? 1 : -1);
        const dataPoint: [string, number, number] = [t, rowIdx, value];
        return value === 0 ? [] : [dataPoint];
      }),
    );

  const sortedRowNames = data.rows
    .toSorted((names1, names2) =>
      rowName(names1).localeCompare(rowName(names2), "en", { numeric: true }),
    )
    .filter((names) => !names.some((name) => name === "Other"))
    .map(rowName);

  return {
    grid: {
      left: 150,
      top: 20,
      right: "1%",
      bottom: 80,
    },
    xAxis: [
      {
        type: "category",
        data: data.t.map((row) => row),
        show: false,
        axisPointer: { show: false },
      },
      {
        type: "time",
        min: data.start,
        max: data.end,
        position: "bottom",
      },
    ],
    yAxis: {
      type: "category",
      data: sortedRowNames,
    },
    visualMap: {
      type: "continuous",
      min: 0,
      max: Math.max.apply(
        Math,
        data["max"].filter((_, rowIdx) =>
          data.rows[rowIdx].some((name) => name !== "Other"),
        ),
      ),
      calculable: true,
      orient: "horizontal",
      right: "5%",
      bottom: 0,
      inRange: {
        color: isDark.value ? PALETTE_MAGMA.toReversed() : PALETTE_MAGMA,
      },
      formatter: (value) => formatXps(value as number),
    },
    dataset: {
      sourceHeader: false,
      dimensions: ["time", ...data.rows.map(rowName)],
      source,
    },
    tooltip: {
      confine: true,
      trigger: "axis",
      axisPointer: {
        type: "cross",
        label: { backgroundColor: "#6a7985" },
      },
      backgroundColor: isDark.value ? "#222e" : "#eeee",
      textStyle: isDark.value ? { color: "#ddd" } : { color: "#222" },
      formatter: () => "",
    },
    series: [
      {
        type: "heatmap",
        data: source,
      },
    ],
  };
});

const { chartComponent, option, updateTimeRange } = useTimeSeriesGraph(
  graph,
  emit,
  toRef(props, "highlight"),
  toRef(props, "data"),
  1,
);
</script>
