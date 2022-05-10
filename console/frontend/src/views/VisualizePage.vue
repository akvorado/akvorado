<template>
  <div class="container mx-auto">
    <ResizeRow
      :slider-width="10"
      :height="graphHeight"
      width="auto"
      slider-bg-color="#eee3"
      slider-bg-hover-color="#ccc3"
    >
      <v-chart :option="graph" autoresize />
    </ResizeRow>
    <div class="relative my-3 overflow-x-auto shadow-md sm:rounded-lg">
      <table class="w-full text-left text-sm text-gray-500 dark:text-gray-400">
        <thead
          class="bg-gray-50 text-xs uppercase text-gray-700 dark:bg-gray-700 dark:text-gray-400"
        >
          <tr>
            <th scope="col" class="px-6 py-2"></th>
            <th
              v-for="column in table.columns"
              :key="column"
              scope="col"
              class="px-6 py-2"
            >
              {{ column }}
            </th>
            <th scope="col" class="px-6 py-2 text-right">Min</th>
            <th scope="col" class="px-6 py-2 text-right">Max</th>
            <th scope="col" class="px-6 py-2 text-right">Average</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in table.rows"
            :key="row.dimensions"
            class="border-b odd:bg-white even:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 odd:dark:bg-gray-800 even:dark:bg-gray-700"
          >
            <th
              scope="row"
              class="px-6 py-2 text-right font-medium text-gray-900 dark:text-white"
            >
              <div class="w-5" :style="row.style">&nbsp;</div>
            </th>
            <td
              v-for="dimension in row.dimensions"
              :key="dimension"
              class="px-6 py-2"
            >
              {{ dimension }}
            </td>
            <td class="px-6 py-2 text-right tabular-nums">
              {{ formatBps(row.min) }}bps
            </td>
            <td class="px-6 py-2 text-right tabular-nums">
              {{ formatBps(row.max) }}bps
            </td>
            <td class="px-6 py-2 text-right tabular-nums">
              {{ formatBps(row.average) }}bps
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, inject } from "vue";
import { notify } from "notiwind";
import { Date as SugarDate } from "sugar-date";
import { use, graphic } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { LineChart } from "echarts/charts";
import { TooltipComponent, GridComponent } from "echarts/components";
import VChart from "vue-echarts";
import { ResizeRow } from "vue-resizer";
import { dataColor, dataColorGrey } from "../utils/palette.js";
const { isDark } = inject("darkMode");

use([CanvasRenderer, LineChart, TooltipComponent, GridComponent]);

const formatBps = (value) => {
  const suffixes = ["", "K", "M", "G", "T"];
  let idx = 0;
  while (value >= 1000 && idx < suffixes.length) {
    value /= 1000;
    idx++;
  }
  value = value.toFixed(2);
  return `${value}${suffixes[idx]}`;
};

const graphHeight = ref(500);
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
const table = ref({
  columns: [],
  rows: [],
});

const request = ref({
  start: SugarDate.create("6 hours ago"),
  end: SugarDate.create("now"),
  points: 100,
  "max-series": 10,
  dimensions: ["SrcAS"],
  filter: {
    operator: "all",
    rules: [
      {
        column: "InIfBoundary",
        operator: "=",
        value: "external",
      },
    ],
  },
});
const fetchedData = ref({});

watch(
  request,
  async () => {
    const response = await fetch("/api/v0/console/graph", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request.value),
    });
    if (!response.ok) {
      notify(
        {
          group: "top",
          kind: "error",
          title: "Unable to fetch data",
          text: `While retrieving data, got a fatal error.`,
        },
        60000
      );
      return;
    }
    const data = await response.json();
    fetchedData.value = data;
  },
  { immediate: true }
);

watch([fetchedData, isDark], ([data, isDark]) => {
  const theme = isDark ? "dark" : "light";

  // Graphic
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
      lineStyle: {
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

  // Table
  table.value = {
    columns: request.value.dimensions,
    rows: data.rows.map((rows, idx) => {
      const color = rows.some((name) => name === "Other")
        ? dataColorGrey
        : dataColor;
      return {
        dimensions: rows,
        style: `background-color: ${color(idx, false, theme)}`,
        min: data.min[idx],
        max: data.max[idx],
        average: data.average[idx],
      };
    }),
  };
});
</script>
