// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import {
  ref,
  watch,
  computed,
  onMounted,
  nextTick,
  type Ref,
  type ComputedRef,
} from "vue";
import { useMediaQuery } from "@vueuse/core";
import { use, graphic, type ComposeOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import {
  LineChart,
  HeatmapChart,
  type LineSeriesOption,
  type HeatmapSeriesOption,
} from "echarts/charts";
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
import VChart from "vue-echarts";

// Register all eCharts components (idempotent)
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

export type ECOption = ComposeOption<
  | LineSeriesOption
  | HeatmapSeriesOption
  | TooltipComponentOption
  | GridComponentOption
  | BrushComponentOption
  | ToolboxComponentOption
  | DatasetComponentOption
  | TitleComponentOption
  | VisualMapComponentOption
>;

export { graphic };

export const rowName = (row: string[]) => row.join(" — ") || "Total";

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

export function useTimeSeriesGraph(
  graph: ComputedRef<ECOption>,
  emit: (event: "update:timeRange", range: [Date, Date]) => void,
  highlight: Ref<number | null>,
  data: Ref<unknown>,
  coordRangeIndex: number = 0,
) {
  const chartComponent = ref<typeof VChart | null>(null);

  const option = computed((): ECOption => ({ ...commonGraph, ...graph.value }));

  // Enable and handle brush
  const isTouchScreen = useMediaQuery("(pointer: coarse)");
  const enableBrush = () => {
    void nextTick().then(() => {
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
  watch([graph, isTouchScreen] as const, enableBrush);

  const updateTimeRange = (evt: BrushModel) => {
    if (
      !chartComponent.value ||
      evt.areas.length === 0 ||
      !evt.areas[0].coordRanges ||
      !evt.areas[0].coordRanges[coordRangeIndex]
    ) {
      return;
    }
    const coordRange = evt.areas[0].coordRanges[coordRangeIndex];
    const [start, end] = coordRange.map((t) => new Date(t as number));
    chartComponent.value.dispatchAction({
      type: "brush",
      areas: [],
    });
    emit("update:timeRange", [start, end]);
  };

  // Highlight selected series
  watch(
    () => [highlight.value, data.value] as const,
    ([index]) => {
      chartComponent.value?.dispatchAction({
        type: "highlight",
        seriesIndex: index,
      });
    },
  );

  return { chartComponent, option, updateTimeRange };
}
