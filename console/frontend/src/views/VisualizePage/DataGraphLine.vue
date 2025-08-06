<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
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
import { ref, watch, inject, computed, onMounted, nextTick } from "vue";
import { useMediaQuery } from "@vueuse/core";
import { formatXps, dataColor, dataColorGrey } from "@/utils";
import { ThemeKey } from "@/components/ThemeProvider.vue";
import type { GraphLineHandlerResult } from ".";
import { uniqWith, isEqual, findIndex } from "lodash-es";
import { use, graphic, type ComposeOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { LineChart, HeatmapChart, type LineSeriesOption, type HeatmapSeriesOption } from "echarts/charts";
import {
  TooltipComponent,
  type TooltipComponentOption,
  GridComponent,
  type GridComponentOption,
  BrushComponent,
  type BrushComponentOption,
  ToolboxComponent,
  type ToolboxComponentOption,
  DatasetComponent,
  type DatasetComponentOption,
  TitleComponent,
  type TitleComponentOption,
  VisualMapComponent,
  type VisualMapComponentOption,
} from "echarts/components";
import type { default as BrushModel } from "echarts/types/src/component/brush/BrushModel.d.ts";
import type { TooltipCallbackDataParams } from "echarts/types/src/component/tooltip/TooltipView.d.ts";
import VChart from "vue-echarts";
use([
  CanvasRenderer,
  LineChart,
  HeatmapChart,
  TooltipComponent,
  GridComponent,
  ToolboxComponent,
  BrushComponent,
  DatasetComponent,
  TitleComponent,
  VisualMapComponent,
]);
type ECOption = ComposeOption<
  | LineSeriesOption
  | HeatmapSeriesOption
  | TooltipComponentOption
  | GridComponentOption
  | BrushComponentOption
  | ToolboxComponentOption
  | DatasetComponentOption
  | TitleComponentOption
>;

const props = defineProps<{
  data: GraphLineHandlerResult;
  highlight: number | null;
}>();
const emit = defineEmits<{
  "update:timeRange": [range: [Date, Date]];
}>();

const { isDark } = inject(ThemeKey)!;

// Graph component
const chartComponent = ref<typeof VChart | null>(null);
const commonGraph: ECOption = {
  backgroundColor: "transparent",
  animationDuration: 500,
  toolbox: {
    show: false,
  },
  brush: {
    xAxisIndex: "all",
  },
};
const graph = computed((): ECOption => {
  const theme = isDark.value ? "dark" : "light";
  const data = props.data;
  if (!data) return {};
  const rowName = (row: string[]) => row.join(" — ") || "Total";
  const source: [number, number, number][] | [string, ...number[]][] = data.graphType === "heatmap" ?
    data.points
      .map((row, rowIdx) => {
        const ret: [number, number[]] = [rowIdx, row];
	return ret;
      })
      .filter(([origRowIdx, row]) =>
        data.rows[origRowIdx].some((name) => name !== "Other")
      )
      .toSorted(([origRowIdx1, row1], [origRowIdx2, row2]) => {
        return rowName(data.rows[origRowIdx1]).localeCompare(rowName(data.rows[origRowIdx2]), "en", { numeric: true });
      })
      .flatMap(([origRowIdx, row], rowIdx) => [
        ...data.t
          .map((t, timeIdx) => {
            let value = row[timeIdx] * (data.axis[origRowIdx] % 2 ? 1 : -1);
            const dataPoint: [number, number, number] = [timeIdx, rowIdx, value];
            return dataPoint;
          }
        )
      ])
  : [
    ...data.t
      .map((t, timeIdx) => {
        let result: [string, ...number[]] = [
          t,
          ...data.points.map(
            // Unfortunately, eCharts does not seem to make it easy
            // to inverse an axis and put the result below. Therefore,
            // we use negative values for the second axis.
            (row, rowIdx) => row[timeIdx] * (data.axis[rowIdx] % 2 ? 1 : -1),
          ),
        ];
        if (data.graphType === "stacked100") {
          // Normalize values between 0 and 1 (or -1 and 0)
          const [, ...values] = result;
          const positiveSum = values.reduce(
            (prev, cur) => (cur > 0 ? prev + cur : prev),
            0,
          );
          const negativeSum = values.reduce(
            (prev, cur) => (cur < 0 ? prev + cur : prev),
            0,
          );
          result = [
            t,
            ...values.map((v) =>
              v > 0 && positiveSum > 0
                ? v / positiveSum
                : v < 0 && negativeSum < 0
                  ? -v / negativeSum
                  : v,
            ),
          ];
        }
        return result;
      })
      .slice(0, -1), // trim last point
  ];
  const dataset = {
      sourceHeader: false,
      dimensions: ["time", ...data.rows.map(rowName)],
      source,
    },
    xAxis: ECOption["xAxis"] = data.graphType === "heatmap" ? {
      type: "category",
      data: data.t.map((row, idx) => row),
    } : {
      type: "time",
      min: data.start,
      max: data.end,
    },
    yAxis: ECOption["yAxis"] = data.graphType === "heatmap" ? {
      type: "category",
      data: data.rows
        .toSorted((names1, names2) => rowName(names1).localeCompare(rowName(names2), "en", { numeric: true }))
        .filter(names => !names.some(name => name === "Other")).map(rowName),
    } : {
      type: "value",
      min: data.bidirectional
        ? data.graphType === "stacked100"
          ? -1
          : undefined
        : 0,
      max: data.graphType === "stacked100" ? 1 : undefined,
      axisLabel: {
        formatter:
          data.graphType === "stacked100"
            ? (v: number) => (v * 100).toFixed(0)
            : ["inl2%", "outl2%"].includes(data.units)
              ? (v: number) => v.toFixed(0)
              : formatXps,
      },
      axisPointer: {
        label: {
          formatter:
            data.graphType === "stacked100"
              ? ({ value }) => ((value.valueOf() as number) * 100).toFixed(1)
              : ["inl2%", "outl2%"].includes(data.units)
                ? ({ value }) => (value.valueOf() as number).toFixed(0)
                : ({ value }) => formatXps(value.valueOf() as number),
        },
      },
    },
    visualMap: ECOption["visualMap"] = data.graphType === "heatmap" ? {
      min: 0,
      max: Math.max.apply(Math, data["max"].filter((_, rowIdx) => data.rows[rowIdx].some((name) => name !== "Other"))),
      calculable: true,
      orient: 'horizontal',
      right: '5%',
      bottom: 0,
      inRange: {
        color: ["#000000", "#5500ff", "#ff4444", "#ff3333", "#ffffff"],
      },
      formatter: formatXps,
    } : undefined,
    tooltip: ECOption["tooltip"] = data.graphType === "heatmap" ? undefined : {
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
        if (!Array.isArray(params) || params.length === 0) return "";

        const table: {
          key: string;
          seriesName: string;
          marker: (typeof params)[0]["marker"];
          up: number;
          down: number;
        }[] = [];
        (params as TooltipCallbackDataParams[]).forEach((param) => {
          if (param.seriesIndex === undefined) return;
          const axis = data.axis[param.seriesIndex];
          const seriesName = [1, 2].includes(axis)
            ? param.seriesName
            : data["axis-names"][axis];
          if (!seriesName) return;
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
          // We need to find the origin value in data.points, notably when using
          // stacked100.
          const val = data.points[param.seriesIndex][param.dataIndex];
          if (axis % 2 == 1) table[idx].up = val;
          else table[idx].down = val;
        });
        const rows = table
          .map((row) =>
            [
              `<tr>`,
              `<td>${row.marker} ${row.seriesName}</td>`,
              `<td class="pl-2">${data.bidirectional ? "↑" : ""}<b>${formatXps(
                row.up,
              )}</b></td>`,
              data.bidirectional
                ? `<td class="pl-2">↓<b>${formatXps(row.down)}</b></td>`
                : "",
              `</tr>`,
            ].join(""),
          )
          .join("");
        return `${
          (params as TooltipCallbackDataParams[])[0].axisValueLabel
        }<table>${rows}</table>`;
      },
    };

  // Lines and stacked areas
  if (
    data.graphType === "stacked" ||
    data.graphType === "stacked100" ||
    data.graphType === "lines" ||
    data.graphType === "heatmap"
  ) {
    const uniqRows = uniqWith(data.rows, isEqual),
      uniqRowIndex = (row: string[]) =>
        findIndex(uniqRows, (orow) => isEqual(row, orow));

    return {
      grid: {
        left: data.graphType === "heatmap" ? 150 : 60,
        top: 20,
        right: "1%",
        bottom: data.graphType === "heatmap" ? 80 : 20,
      },
      xAxis,
      yAxis,
      visualMap,
      dataset,
      tooltip,
      series: data.graphType === "heatmap" ? [
        {
          type: 'heatmap',
          data: source,
        }
      ] : data.rows
        .map((row, idx) => {
          const isOther = row.some((name) => name === "Other"),
            color = isOther ? dataColorGrey : dataColor;
          if (data.graphType === "lines" && isOther) {
            return undefined;
          }
          let serie: LineSeriesOption = {
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
            (data.graphType === "stacked" || data.graphType === "stacked100") &&
            [1, 2].includes(data.axis[idx])
          ) {
            serie = {
              ...serie,
              stack: data.axis[idx].toString(),
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
        .filter((s): s is LineSeriesOption => !!s),
    };
  }
  if (data.graphType === "grid") {
    const uniqRows = uniqWith(data.rows, isEqual).filter((row) =>
        row.some((name) => name !== "Other"),
      ),
      uniqRowIndex = (row: string[]) =>
        findIndex(uniqRows, (orow) => isEqual(row, orow)),
      otherIndexes = data.rows
        .map((row, idx) => (row.some((name) => name === "Other") ? idx : -1))
        .filter((idx) => idx >= 0),
      somethingY = (fn: (...n: number[]) => number) =>
        fn(
          ...dataset.source.map((row) => {
            const [, ...cdr] = row;
            return fn(
              ...cdr.filter((_, idx) => !otherIndexes.includes(idx + 1)),
            );
          }),
        ),
      maxY = somethingY(Math.max),
      minY = somethingY(Math.min);
    let rowNumber = Math.ceil(Math.sqrt(uniqRows.length));
    const colNumber = rowNumber;
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
          const serie: LineSeriesOption = {
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
        .filter((s) => s.xAxisIndex! >= 0),
    };
  }
  return {};
});
const option = computed((): ECOption => ({ ...commonGraph, ...graph.value }));

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
const updateTimeRange = (evt: BrushModel) => {
  if (
    !chartComponent.value ||
    evt.areas.length === 0 ||
    !evt.areas[0].coordRange
  ) {
    return;
  }
  const [start, end] = evt.areas[0].coordRange.map(
    (t) => new Date(t as number),
  );
  chartComponent.value.dispatchAction({
    type: "brush",
    areas: [],
  });
  emit("update:timeRange", [start, end]);
};
watch([graph, isTouchScreen] as const, enableBrush);

// Highlight selected indexes
watch(
  () => [props.highlight, props.data] as const,
  ([index]) => {
    chartComponent.value?.dispatchAction({
      type: "highlight",
      seriesIndex: index,
    });
  },
);
</script>
